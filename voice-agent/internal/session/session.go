package session

import (
	"context"
	"sync"
	"time"

	"timbre/voice-agent/internal/amd"
)

// Event is the structured message exchanged with the WebSocket client.
// Binary audio frames are sent separately on the same socket.
type Event struct {
	Type    string `json:"type"`
	Role    string `json:"role,omitempty"`
	Text    string `json:"text,omitempty"`
	Final   bool   `json:"final,omitempty"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Config is the per-session bot configuration handed in at session creation time.
type Config struct {
	CallID     string   `json:"callId"`
	TenantID   string   `json:"tenantId"`
	BotID      string   `json:"botId"`
	Provider   string   `json:"provider"`
	Objective  string   `json:"objective"`
	Guardrails []string `json:"guardrails"`
	Language   string   `json:"language"`
	Voice      string   `json:"voice"`
	LeadName   string   `json:"leadName,omitempty"`

	// Credentials are per-tenant overrides for the provider keys/models. Empty fields fall back
	// to the voice-agent's env defaults.
	Credentials Credentials `json:"credentials,omitempty"`

	// Tools (function calling) que el LLM del provider puede invocar.
	// Los pasamos verbatim al negociar Settings (Deepgram) o
	// session.update.tools (OpenAI). Cuando el provider emita una
	// invocación, el provider implementation llama a InvokeTool del
	// session para que se ejecute via backend.
	Tools []Tool `json:"tools,omitempty"`

	// AMD (Answering Machine Detection). Si AMD.Enabled es true, el
	// audiosocket alimenta un detector con los primeros segundos de audio
	// y, según AMD.Action, decide si colgar o soltar un mensaje TTS.
	AMD AMDConfig `json:"amd,omitempty"`
}

// AMDConfig es la configuración de detección de buzón para la sesión.
// Action posibles:
//   - "hangup"       — al detectar machine, cerrar sesión inmediatamente.
//   - "drop_message" — recitar Message (TTS via provider) y cerrar.
//   - "continue"     — solo registrar el resultado, no actuar (debug).
type AMDConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Action  string `json:"action,omitempty"`
	Message string `json:"message,omitempty"`
}

// Tool es una function exponible al LLM. parameters es JSON Schema.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Credentials struct {
	OpenAIAPIKey        string `json:"openaiApiKey,omitempty"`
	OpenAIRealtimeModel string `json:"openaiRealtimeModel,omitempty"`
	OpenAIRealtimeVoice string `json:"openaiRealtimeVoice,omitempty"`

	DeepgramAPIKey        string `json:"deepgramApiKey,omitempty"`
	DeepgramListenModel   string `json:"deepgramListenModel,omitempty"`
	DeepgramThinkProvider string `json:"deepgramThinkProvider,omitempty"`
	DeepgramThinkModel    string `json:"deepgramThinkModel,omitempty"`
	DeepgramSpeakModel    string `json:"deepgramSpeakModel,omitempty"`
	DeepgramGreeting      string `json:"deepgramGreeting,omitempty"`

	AssemblyAIAPIKey   string `json:"assemblyaiApiKey,omitempty"`
	AssemblyAIVoice    string `json:"assemblyaiVoice,omitempty"`
	AssemblyAIGreeting string `json:"assemblyaiGreeting,omitempty"`

	// ElevenLabs Conversational AI. AgentID se configura por bot (no
	// es per-tenant) y va aquí para que el provider lo resuelva como
	// los demás.
	ElevenLabsAPIKey  string `json:"elevenlabsApiKey,omitempty"`
	ElevenLabsAgentID string `json:"elevenlabsAgentId,omitempty"`
}

// Session is the in-memory representation of a live voice conversation. The provider's Run loop
// reads from AudioIn, pushes audio to AudioOut, and emits Events. The WebSocket handler bridges
// these channels with the network connection.
type Session struct {
	ID        string
	Config    Config
	CreatedAt time.Time

	AudioIn  chan []byte
	AudioOut chan []byte
	Events   chan Event

	cancel       context.CancelFunc
	ctx          context.Context
	mu           sync.Mutex
	closed       bool
	onClose      func()
	onTranscript func(sessionID, role, text string)
	onToolInvoke func(ctx context.Context, sessionID, toolName string, args map[string]any) (content string, ok bool)
	onAMDResult  func(sessionID string, result amd.Result)

	amdDet      *amd.Detector // nil si AMD deshabilitado
	amdResult   amd.Result    // último resultado conocido, "" si pending
	amdNotified bool          // garantiza que onAMDResult corre una sola vez

	// Last transcripts, for inspection via HTTP API.
	transcript []TranscriptLine
}

type TranscriptLine struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

func New(id string, cfg Config) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		ID:        id,
		Config:    cfg,
		CreatedAt: time.Now().UTC(),
		AudioIn:   make(chan []byte, 64),
		AudioOut:  make(chan []byte, 64),
		Events:    make(chan Event, 64),
		ctx:       ctx,
		cancel:    cancel,
	}
	if cfg.AMD.Enabled {
		s.amdDet = amd.New()
	}
	return s
}

func (s *Session) Context() context.Context { return s.ctx }

// Close terminates the session. Safe to call multiple times.
func (s *Session) Close(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	// Emit a terminal event for whoever is listening.
	select {
	case s.Events <- Event{Type: "end", Reason: reason}:
	default:
	}
	s.cancel()
	if s.onClose != nil {
		s.onClose()
	}
	close(s.AudioIn)
	close(s.AudioOut)
	close(s.Events)
}

func (s *Session) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Session) SetOnClose(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onClose = fn
}

func (s *Session) AppendTranscript(role, text string) {
	s.mu.Lock()
	s.transcript = append(s.transcript, TranscriptLine{Role: role, Text: text, At: time.Now().UTC()})
	if len(s.transcript) > 500 {
		s.transcript = s.transcript[len(s.transcript)-500:]
	}
	hook := s.onTranscript
	s.mu.Unlock()
	if hook != nil {
		// Fire-and-forget. The hook implementation must be non-blocking or return quickly.
		go hook(s.ID, role, text)
	}
}

// SetOnTranscript installs a callback fired after every persisted transcript line. Used by the
// HTTP API to wire transcripts to the backend webhook.
func (s *Session) SetOnTranscript(fn func(sessionID, role, text string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onTranscript = fn
}

// SetOnToolInvoke installs a callback used by providers to dispatch function
// calls coming from the LLM. fn devuelve el "content" textual que el provider
// reenviará al LLM y un bool indicando si la invocación se ejecutó OK. Si
// el handler está sin instalar (no hay backend), devolver ok=false para que
// el provider responda al LLM con un fallback genérico.
func (s *Session) SetOnToolInvoke(fn func(ctx context.Context, sessionID, toolName string, args map[string]any) (string, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onToolInvoke = fn
}

// InvokeTool dispatcha al callback registrado o devuelve un fallback. Se
// llama desde providers cuando reciben un function_call event.
func (s *Session) InvokeTool(ctx context.Context, toolName string, args map[string]any) (string, bool) {
	s.mu.Lock()
	hook := s.onToolInvoke
	s.mu.Unlock()
	if hook == nil {
		return "Action unavailable.", false
	}
	return hook(ctx, s.ID, toolName, args)
}

// SetOnAMDResult instala el callback que se dispara cuando el detector
// AMD emite un veredicto (human/machine/unknown). Solo se llama una vez.
// El callback debe ser no-bloqueante; aquí se dispara en una goroutine.
func (s *Session) SetOnAMDResult(fn func(sessionID string, result amd.Result)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAMDResult = fn
}

// ObserveInbound se llama desde el audiosocket por cada frame de audio
// recibido del caller. Si AMD está habilitado y todavía no ha decidido,
// alimenta el detector. Cuando el detector llega a veredicto, dispara
// el callback registrado.
//
// Mantenerla barata — corre en el read loop del audiosocket.
func (s *Session) ObserveInbound(pcm []byte) {
	s.mu.Lock()
	det := s.amdDet
	if det == nil || s.amdNotified {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	det.FeedPCM(pcm)
	res, ok := det.Result()
	if !ok {
		return
	}
	s.mu.Lock()
	if s.amdNotified {
		s.mu.Unlock()
		return
	}
	s.amdNotified = true
	s.amdResult = res
	hook := s.onAMDResult
	s.mu.Unlock()
	if hook != nil {
		go hook(s.ID, res)
	}
}

// AMDResult devuelve el último veredicto conocido del detector ("" si
// todavía no decidió o si AMD estaba deshabilitado).
func (s *Session) AMDResult() amd.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.amdResult
}

func (s *Session) Snapshot() ([]TranscriptLine, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]TranscriptLine, len(s.transcript))
	copy(cp, s.transcript)
	return cp, s.CreatedAt, s.closed
}

// Registry holds active sessions keyed by ID, with TTL-based cleanup.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewRegistry(ttl time.Duration) *Registry {
	r := &Registry{sessions: map[string]*Session{}, ttl: ttl}
	go r.gc()
	return r
}

func (r *Registry) Add(s *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = s
}

func (r *Registry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[id]; ok {
		s.Close("removed")
		delete(r.sessions, id)
	}
}

func (r *Registry) List() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}

func (r *Registry) gc() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for range tick.C {
		cutoff := time.Now().Add(-r.ttl)
		r.mu.Lock()
		for id, s := range r.sessions {
			if s.CreatedAt.Before(cutoff) || s.Closed() {
				s.Close("ttl_expired")
				delete(r.sessions, id)
			}
		}
		r.mu.Unlock()
	}
}
