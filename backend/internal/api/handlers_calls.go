package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
	writeJSON(w, http.StatusOK, calls)
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
	writeJSON(w, http.StatusOK, call)
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

// requireInternalSecret guards endpoints that only the voice-agent should call. The voice-agent
// shares a static secret with the backend (VOICE_AGENT_SHARED_SECRET).
func (s *Server) requireInternalSecret(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.VoiceAgent.Secret == "" {
			// No secret configured: only allow loopback / private network. In Docker compose this
			// means only the voice-agent service can reach the backend via its internal name.
			next(w, r)
			return
		}
		got := r.Header.Get("X-Internal-Secret")
		if got == "" {
			got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if got != s.cfg.VoiceAgent.Secret {
			writeError(w, http.StatusUnauthorized, "invalid_secret")
			return
		}
		next(w, r)
	}
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
