package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/amd"
	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/provider"
	"timbre/voice-agent/internal/recording"
	"timbre/voice-agent/internal/rtp"
	"timbre/voice-agent/internal/session"
	"timbre/voice-agent/internal/webhook"
)

type Server struct {
	cfg       config.Config
	registry  *session.Registry
	providers *provider.Registry
	webhook   *webhook.Client
	rtpPool   *rtp.Pool
	logger    *slog.Logger
}

func New(cfg config.Config, reg *session.Registry, providers *provider.Registry, wh *webhook.Client, pool *rtp.Pool, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, registry: reg, providers: providers, webhook: wh, rtpPool: pool, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /providers", s.handleProviders)
	mux.HandleFunc("POST /sessions", s.requireSharedSecret(s.handleCreateSession))
	mux.HandleFunc("GET /sessions", s.requireSharedSecret(s.handleListSessions))
	mux.HandleFunc("GET /sessions/{id}", s.requireSharedSecret(s.handleGetSession))
	mux.HandleFunc("DELETE /sessions/{id}", s.requireSharedSecret(s.handleEndSession))
	mux.HandleFunc("GET /sessions/{id}/audio", s.handleAudioWS)
	mux.HandleFunc("POST /sessions/{id}/rtp", s.requireSharedSecret(s.handleAllocateRTP))
	return s.cors(s.logRequests(mux))
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Voice-Agent-Secret")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if slices.Contains(s.cfg.AllowedOrigins, "*") {
		return true
	}
	return slices.Contains(s.cfg.AllowedOrigins, origin)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

// requireSharedSecret protects management endpoints (create/list/delete sessions). The audio
// WebSocket is intentionally public so RTP-bridge processes (or browser test clients) can connect
// using only the session id.
func (s *Server) requireSharedSecret(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.BackendAuthKey == "" {
			next(w, r) // not configured: open to local network only (compose binds limit exposure)
			return
		}
		got := r.Header.Get("X-Voice-Agent-Secret")
		if got == "" {
			got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if got != s.cfg.BackendAuthKey {
			writeError(w, http.StatusUnauthorized, "invalid_secret")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"providers": s.providers.Names(),
		"sessions":  len(s.registry.List()),
		"time":      time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.providers.Names()})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var cfg session.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if cfg.Provider == "" {
		cfg.Provider = "echo"
	}
	prov, ok := s.providers.Get(cfg.Provider)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown_or_unconfigured_provider")
		return
	}
	sess := session.New(newSessionID(), cfg)
	if s.webhook != nil && s.webhook.Enabled() {
		sess.SetOnTranscript(func(sessionID, role, text string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			s.webhook.PostTranscript(ctx, webhook.TranscriptInput{SessionID: sessionID, Role: role, Text: text})
		})
		// Tools: cuando un provider emita un function_call, delegamos al backend
		// que conoce la action_type/action_config y ejecuta la acción real.
		sess.SetOnToolInvoke(func(ctx context.Context, sessionID, toolName string, args map[string]any) (string, bool) {
			r, err := s.webhook.InvokeTool(ctx, webhook.ToolInvokeInput{
				SessionID: sessionID, ToolName: toolName, Arguments: args,
			})
			if err != nil {
				s.logger.Warn("tool invoke failed", "session", sessionID, "tool", toolName, "error", err)
			}
			return r.Content, r.Success
		})
		// AMD: cuando el detector emite veredicto avisamos al backend.
		// El backend persiste calls.amd_result y, si amd_action=hangup,
		// dispara DELETE /sessions/{id} para cerrar la sesión.
		// Al cerrar la sesión, reportamos el consumo. Solo conocemos
		// DurationSec con seguridad — el resto (tokens, chars) lo
		// rellenará la futura instrumentación por provider.
		startedAt := time.Now()
		sessID := sess.ID
		sess.SetOnClose(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			dur := int(time.Since(startedAt).Seconds())
			s.webhook.PostUsage(ctx, webhook.UsageInput{
				SessionID:   sessID,
				DurationSec: dur,
				STTSeconds:  dur, // asumimos que escuchamos toda la llamada
			})
		})
		botID := cfg.BotID
		sess.SetOnAMDResult(func(sessionID string, result amd.Result) {
			s.logger.Info("amd verdict", "session", sessionID, "result", string(result))
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			s.webhook.PostAMDResult(ctx, webhook.AMDResultInput{
				SessionID: sessionID, BotID: botID, Result: string(result),
			})
		})
	}
	s.registry.Add(sess)
	// Launch provider in background; it ends when the session is closed.
	go func() {
		defer s.registry.Remove(sess.ID)
		if err := prov.Run(sess.Context(), sess); err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Warn("provider exited", "session", sess.ID, "provider", prov.Name(), "error", err)
		}
	}()
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          sess.ID,
		"provider":    prov.Name(),
		"audioWsPath": "/sessions/" + sess.ID + "/audio",
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	out := []map[string]any{}
	for _, sess := range s.registry.List() {
		transcript, createdAt, closed := sess.Snapshot()
		out = append(out, map[string]any{
			"id":         sess.ID,
			"callId":     sess.Config.CallID,
			"tenantId":   sess.Config.TenantID,
			"botId":      sess.Config.BotID,
			"provider":   sess.Config.Provider,
			"createdAt":  createdAt,
			"closed":     closed,
			"turns":      len(transcript),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.registry.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found")
		return
	}
	transcript, createdAt, closed := sess.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         sess.ID,
		"config":     sess.Config,
		"createdAt":  createdAt,
		"closed":     closed,
		"transcript": transcript,
	})
}

func (s *Server) handleEndSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.registry.Remove(id)
	w.WriteHeader(http.StatusNoContent)
}

// handleAudioWS bridges the session's audio channels with a WebSocket. Binary frames carry PCM16
// audio in both directions; text frames carry session.Event JSON. Anyone with the session id can
// connect — typical caller is an RTP gateway bridging Asterisk External Media, but a browser test
// client also works.
func (s *Server) handleAudioWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.registry.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})
	if err != nil {
		s.logger.Warn("ws accept", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "client_closed")
	conn.SetReadLimit(4 * 1024 * 1024)

	ctx, cancel := context.WithCancel(sess.Context())
	defer cancel()

	// Per-connection recorder. Tees both inbound and outbound audio into one mono WAV
	// y al cerrar la sesión lo comprime con ffmpeg → OGG/Opus 24 kbps voip.
	// PCM16 16 kHz mono = ~1.92 MB/min; Opus 24 kbps = ~180 KB/min (~10x menos).
	// Si ffmpeg no está, fallback automático a WAV.
	//
	// Backing por archivo temp en disco — antes acumulábamos en RAM con
	// cap 20 MB y llamadas >10 min se truncaban silenciosamente. El
	// archivo .pcm vive hasta rec.Close() en defer (corre DESPUÉS del
	// upload, por orden LIFO).
	rec := recording.New(sess.ID, 16000)
	defer func() {
		if err := rec.Close(); err != nil {
			s.logger.Warn("recorder close", "session", sess.ID, "error", err)
		}
	}()
	defer func() {
		if s.cfg.BackendURL == "" {
			return
		}
		uploadCtx, ucancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer ucancel()

		// Intentamos Opus primero; si ffmpeg no está, caemos a WAV.
		body, contentType, err := rec.EncodeOpus(uploadCtx)
		if err != nil && !errors.Is(err, recording.ErrNoFFmpeg) {
			s.logger.Warn("recording opus encode failed, falling back to wav",
				"session", sess.ID, "error", err)
			body = nil
		}
		// El "raw_wav_bytes" del log es el tamaño que tendría el WAV
		// equivalente (44 B header + PCM total). BytesWritten cuenta el
		// PCM acumulado sin tener que volver a leer el archivo entero.
		wavSize := int(rec.BytesWritten()) + 44
		if body == nil {
			body = rec.WAV()
			contentType = "audio/wav"
		}
		if len(body) == 0 {
			return
		}
		up := recording.NewUploader(s.cfg.BackendURL, s.cfg.BackendAuthKey)
		if err := up.Upload(uploadCtx, sess.ID, body, contentType); err != nil {
			s.logger.Warn("recording upload", "session", sess.ID, "error", err)
		} else {
			s.logger.Info("recording uploaded",
				"session", sess.ID,
				"format", contentType,
				"bytes", len(body),
				"raw_wav_bytes", wavSize,
				"compression_ratio", fmtRatio(wavSize, len(body)))
		}
	}()

	// Audio + events out: drain session.AudioOut and session.Events into the socket. Each
	// outbound audio chunk is also fed into the recorder.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-sess.AudioOut:
				if !ok {
					return
				}
				rec.Append(chunk)
				_ = conn.Write(ctx, websocket.MessageBinary, chunk)
			case ev, ok := <-sess.Events:
				if !ok {
					return
				}
				data, _ := json.Marshal(ev)
				_ = conn.Write(ctx, websocket.MessageText, data)
			}
		}
	}()

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch typ {
		case websocket.MessageBinary:
			rec.Append(data)
			select {
			case sess.AudioIn <- data:
			case <-ctx.Done():
				return
			}
		case websocket.MessageText:
			var msg struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(data, &msg)
			if msg.Type == "stop" {
				sess.Close("client_stop")
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// fmtRatio formatea WAV→Opus como "16.5x" para el log. Útil al revisar
// si la compresión está dando el ahorro esperado por sesión.
func fmtRatio(rawBytes, encodedBytes int) string {
	if encodedBytes <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1fx", float64(rawBytes)/float64(encodedBytes))
}
