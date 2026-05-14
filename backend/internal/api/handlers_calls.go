package api

import (
	"crypto/subtle"
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
