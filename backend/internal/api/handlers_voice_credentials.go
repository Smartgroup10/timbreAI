package api

import (
	"encoding/json"
	"net/http"

	"timbre/backend/internal/store"
)

func (s *Server) handleGetVoiceCredentials(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	creds, err := s.store.GetVoiceCredentials(r.Context(), tenantID)
	if err != nil {
		s.logger.Error("voice credentials get", "error", err)
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	// Always return masked keys so the UI never sees the real value after save.
	writeJSON(w, http.StatusOK, creds.Masked())
}

func (s *Server) handleUpdateVoiceCredentials(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var p store.VoiceCredentialsPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	// Drop "no change" sentinels — UI sends back the masked value untouched if the user didn't
	// rotate the key. We treat any masked-looking value as "leave as is".
	stripMasked := func(v *string) *string {
		if v == nil {
			return nil
		}
		if isMaskedSecret(*v) {
			return nil
		}
		return v
	}
	p.OpenAIAPIKey = stripMasked(p.OpenAIAPIKey)
	p.DeepgramAPIKey = stripMasked(p.DeepgramAPIKey)
	p.AssemblyAIAPIKey = stripMasked(p.AssemblyAIAPIKey)

	creds, err := s.store.UpdateVoiceCredentials(r.Context(), tenantID, p)
	if err != nil {
		s.logger.Error("voice credentials update", "error", err)
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	payload := map[string]any{}
	if p.OpenAIAPIKey != nil {
		payload["openaiApiKey"] = "rotated"
	}
	if p.DeepgramAPIKey != nil {
		payload["deepgramApiKey"] = "rotated"
	}
	if p.AssemblyAIAPIKey != nil {
		payload["assemblyaiApiKey"] = "rotated"
	}
	s.audit(r, "voice_credentials.update", "voice_credentials", tenantID, payload)
	writeJSON(w, http.StatusOK, creds.Masked())
}

func isMaskedSecret(s string) bool {
	if s == "" {
		return false
	}
	// Anything that starts with the bullet character • is treated as the mask.
	for _, r := range s {
		if r == '•' {
			return true
		}
		break
	}
	return false
}
