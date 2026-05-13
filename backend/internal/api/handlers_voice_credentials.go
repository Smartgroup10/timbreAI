package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"timbre/backend/internal/store"
	"timbre/backend/internal/voicecatalog"
)

// handleGetVoiceCatalog devuelve la lista estática de providers + voces +
// modelos disponibles. Lo usa la UI de edición de bots para poblar los
// dropdowns. No es secreto y por tanto cualquier usuario autenticado puede
// leerlo.
func (s *Server) handleGetVoiceCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": voicecatalog.All})
}

// handleTestVoiceCredentials pinga al provider con la API key configurada
// para el tenant y reporta éxito/fallo. Operación rápida (~2s), no consume
// tokens (no manda audio ni completion).
//
// Body: { "provider": "openai_realtime" | "deepgram" | "assemblyai" }
func (s *Server) handleTestVoiceCredentials(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider_required")
		return
	}
	creds, err := s.store.GetVoiceCredentials(r.Context(), tenantID)
	if err != nil {
		s.logger.Error("voice credentials get", "error", err)
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var probeErr error
	switch provider {
	case "openai_realtime":
		if creds.OpenAIAPIKey == "" {
			probeErr = errKeyMissing
			break
		}
		probeErr = pingOpenAI(ctx, creds.OpenAIAPIKey)
	case "deepgram":
		if creds.DeepgramAPIKey == "" {
			probeErr = errKeyMissing
			break
		}
		probeErr = pingDeepgram(ctx, creds.DeepgramAPIKey)
	case "assemblyai":
		if creds.AssemblyAIAPIKey == "" {
			probeErr = errKeyMissing
			break
		}
		probeErr = pingAssemblyAI(ctx, creds.AssemblyAIAPIKey)
	default:
		writeError(w, http.StatusBadRequest, "unknown_provider")
		return
	}
	if probeErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": probeErr.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

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
