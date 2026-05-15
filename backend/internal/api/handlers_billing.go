package api

// Billing & coste real por llamada.
//
// El voice-agent reporta al cerrar una sesión los contadores reales que
// ha consumido (segundos STT, tokens LLM in/out, chars TTS, segundos
// TTS). El backend usa las tarifas detalladas (provider_rates si están,
// si no defaults del paquete pricing) para convertir esos contadores en
// micro-céntimos y persiste el row en call_usage. El dashboard agrega
// por día y por dimensión (campaign|provider).

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"timbre/backend/internal/store"
)

type internalUsageInput struct {
	SessionID       string `json:"sessionId"`
	DurationSec     int    `json:"durationSec"`
	STTSeconds      int    `json:"sttSeconds"`
	LLMInputTokens  int    `json:"llmInputTokens"`
	LLMOutputTokens int    `json:"llmOutputTokens"`
	TTSChars        int    `json:"ttsChars"`
	TTSSeconds      int    `json:"ttsSeconds"`
}

// handleInternalUsage recibe del voice-agent los contadores al cerrar la
// sesión. Computa el coste por componente con las tarifas detalladas y
// hace upsert en call_usage. Si algunos contadores vienen a 0 (provider
// no reporta), caemos al flat cents/min del paquete pricing como
// fallback para no perder visibilidad.
func (s *Server) handleInternalUsage(w http.ResponseWriter, r *http.Request) {
	var in internalUsageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if in.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_required")
		return
	}
	call, err := s.store.FindCallByVoiceSession(r.Context(), in.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// La call puede no estar linkeada todavía si el voice-agent
			// reporta inmediato. Aceptamos sin persistir.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}

	provider := call.Provider
	u := store.CallUsage{
		CallID:          call.ID,
		TenantID:        call.TenantID,
		Provider:        provider,
		DurationSec:     in.DurationSec,
		STTSeconds:      in.STTSeconds,
		LLMInputTokens:  in.LLMInputTokens,
		LLMOutputTokens: in.LLMOutputTokens,
		TTSChars:        in.TTSChars,
		TTSSeconds:      in.TTSSeconds,
	}

	// Coste por componente con tarifas detalladas.
	if s.detailed != nil {
		u.STTMicroCents = s.detailed.CostByComponent(provider, "stt", in.STTSeconds)
		u.LLMMicroCents = s.detailed.CostByComponent(provider, "llm_input", in.LLMInputTokens) +
			s.detailed.CostByComponent(provider, "llm_output", in.LLMOutputTokens)
		// TTS: si el provider factura por char usamos chars; si por sec, usamos sec.
		// Aplicamos ambos y nos quedamos con el mayor (cubre los dos casos).
		ttsByChar := s.detailed.CostByComponent(provider, "tts", in.TTSChars)
		ttsBySec := int64(0)
		if in.TTSSeconds > 0 {
			ttsBySec = s.detailed.CostByComponent(provider, "tts_sec", in.TTSSeconds)
		}
		if ttsBySec > ttsByChar {
			u.TTSMicroCents = ttsBySec
		} else {
			u.TTSMicroCents = ttsByChar
		}
	}

	// Fallback: si no hay nada detallado, usamos flat cents/min para
	// que el dashboard no muestre 0€ a llamadas reales.
	if u.STTMicroCents == 0 && u.LLMMicroCents == 0 && u.TTSMicroCents == 0 {
		flatCents := s.pricing.Cost(provider, in.DurationSec)
		// 1 centavo = 10_000 micro-céntimos.
		u.OtherMicroCents = int64(flatCents) * 10_000
	}

	if err := s.store.UpsertCallUsage(r.Context(), u); err != nil {
		s.logger.Error("call usage upsert", "call", call.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "persist_failed")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleBillingSummary devuelve el agregado diario para el tenant
// del JWT. Acepta:
//
//	?from=YYYY-MM-DD  (default: hoy - 30d)
//	?to=YYYY-MM-DD    (default: hoy + 1d, exclusivo)
//	?groupBy=campaign|provider  (default: none)
//
// Las cifras se devuelven en micro-céntimos — el frontend los pasa a EUR/USD.
func (s *Server) handleBillingSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	q := r.URL.Query()
	from, to := parseRange(q.Get("from"), q.Get("to"))
	groupBy := q.Get("groupBy")
	rows, err := s.store.BillingSummary(r.Context(), tenantID, groupBy, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "summary_failed")
		return
	}
	// Total agregado de toda la ventana, para el "card" superior del dashboard.
	var totalCalls int
	var totalDuration, totalMicro int64
	for _, r := range rows {
		totalCalls += r.Calls
		totalDuration += r.DurationSec
		totalMicro += r.TotalMicroCents
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":            from,
		"to":              to,
		"groupBy":         groupBy,
		"rows":            rows,
		"totalCalls":      totalCalls,
		"totalDurationSec": totalDuration,
		"totalMicroCents": totalMicro,
	})
}

// handleBillingCall devuelve el breakdown de una sola llamada para la
// vista de detalle (drawer / tooltip).
func (s *Server) handleBillingCall(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	usage, err := s.store.GetCallUsage(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "usage_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

// parseRange normaliza los parámetros from/to del summary. Si vienen
// vacíos o mal formados, usamos un default razonable (últimos 30 días).
func parseRange(fromStr, toStr string) (time.Time, time.Time) {
	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	from := to.Add(-30 * 24 * time.Hour)
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24 * time.Hour) // exclusivo
		}
	}
	return from, to
}
