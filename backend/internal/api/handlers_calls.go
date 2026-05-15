package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"timbre/backend/internal/store"
)

func (s *Server) handleGetLead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	lead, err := s.store.GetLead(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "lead_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, lead)
}

func (s *Server) handleLeadCalls(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	calls, err := s.store.ListCallsForLead(r.Context(), tenantID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, s.withCost(calls))
}

func (s *Server) handleGetCall(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	call, err := s.store.GetCall(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "call_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, s.withCostOne(call))
}

func (s *Server) handleCallTranscripts(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	transcripts, err := s.store.ListCallTranscripts(r.Context(), tenantID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, transcripts)
}

// --- Internal webhook from voice-agent ---

type internalTranscriptInput struct {
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Text      string `json:"text"`
}

// requireInternalSecret guards endpoints que solo el voice-agent debe llamar.
// El secret es obligatorio en config (Load rechaza arrancar sin él), así que
// aquí simplemente lo comparamos en tiempo constante. SIN fallback "permitir
// todo si secret==''" — eso era un foot-gun del código original.
func (s *Server) requireInternalSecret(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Internal-Secret")
		if got == "" {
			got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		expected := s.cfg.VoiceAgent.Secret
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid_secret")
			return
		}
		next(w, r)
	}
}

// handleInternalAMDResult recibe del voice-agent el veredicto del detector
// AMD. Persiste calls.amd_result y, si la config del bot es amd_action=hangup,
// cierra la sesión vía DELETE /sessions/{id} del voice-agent.
//
// drop_message: se marca voicemail_dropped=true preventivamente — el LLM
// tendrá instrucciones extra (system) para soltar el mensaje cuando lo
// reconozca. Si el resultado es human o unknown no actuamos.
func (s *Server) handleInternalAMDResult(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SessionID string `json:"sessionId"`
		BotID     string `json:"botId"`
		Result    string `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if input.SessionID == "" || input.Result == "" {
		writeError(w, http.StatusBadRequest, "session_and_result_required")
		return
	}
	// La call enlazada nos da el id para persistir. Si todavía no se ha
	// linkado el voice_session_id (race muy corto al inicio), aceptamos
	// igualmente y solo loggeamos.
	call, err := s.store.FindCallByVoiceSession(r.Context(), input.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	// Recuperamos el bot (por botId del payload) para decidir la acción.
	var voicemailDropped bool
	var action string
	if input.BotID != "" {
		if bot, err := s.store.GetBotByID(r.Context(), input.BotID); err == nil {
			action = bot.AMDAction
			if input.Result == "machine" && action == "drop_message" && bot.VoicemailMessage != "" {
				voicemailDropped = true
			}
		}
	}
	if err := s.store.UpdateCallAMD(r.Context(), call.ID, input.Result, voicemailDropped); err != nil {
		s.logger.Error("amd persist", "call", call.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "persist_failed")
		return
	}
	// Si action=hangup y detectamos machine: cerrar sesión del voice-agent
	// (provoca también el cierre del Stasis bridge desde el lado del bot).
	if input.Result == "machine" && action == "hangup" && s.voiceAgent != nil && s.voiceAgent.Enabled() {
		go func(sessionID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.voiceAgent.EndSession(ctx, sessionID); err != nil {
				s.logger.Warn("amd hangup end session", "session", sessionID, "error", err)
			}
		}(input.SessionID)
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleInternalTranscript(w http.ResponseWriter, r *http.Request) {
	var input internalTranscriptInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if input.SessionID == "" || input.Text == "" {
		writeError(w, http.StatusBadRequest, "session_and_text_required")
		return
	}
	if input.Role == "" {
		input.Role = "user"
	}
	call, err := s.store.FindCallByVoiceSession(r.Context(), input.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session_not_linked")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	if err := s.store.AppendTranscript(r.Context(), call.TenantID, call.ID, input.Role, input.Text); err != nil {
		s.logger.Error("append transcript", "error", err)
		writeError(w, http.StatusInternalServerError, "persist_failed")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
