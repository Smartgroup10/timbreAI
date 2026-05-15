package api

// DID routing rules — CRUD por DID.
//
// Endpoints:
//   GET    /api/admin/dids/{didId}/routing-rules            — listar
//   POST   /api/admin/dids/{didId}/routing-rules            — crear
//   PATCH  /api/admin/dids/{didId}/routing-rules/{ruleId}   — editar
//   DELETE /api/admin/dids/{didId}/routing-rules/{ruleId}   — borrar
//
// Solo platform_admin las edita por ahora (viven en /admin/trunks).
// El tenant las verá pero no podrá tocarlas — futura iteración si vemos
// que los operadores quieren autogestionarse el horario.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"timbre/backend/internal/store"
)

type didRoutingRuleInput struct {
	Name           string   `json:"name"`
	Priority       *int     `json:"priority,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	Timezone       string   `json:"timezone"`
	DaysOfWeek     []int    `json:"daysOfWeek"`
	StartMinute    *int     `json:"startMinute,omitempty"`
	EndMinute      *int     `json:"endMinute,omitempty"`
	CallerPrefixes []string `json:"callerPrefixes"`
	Language       string   `json:"language"`
	TargetBotID    string   `json:"targetBotId"`
	FallbackBotID  *string  `json:"fallbackBotId,omitempty"`
}

func (s *Server) handleListDIDRoutingRules(w http.ResponseWriter, r *http.Request) {
	tenantID, didID, err := s.routingScope(r)
	if err != nil {
		writeError(w, err.code, err.msg)
		return
	}
	rules, err2 := s.store.ListDIDRoutingRules(r.Context(), tenantID, didID)
	if err2 != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleCreateDIDRoutingRule(w http.ResponseWriter, r *http.Request) {
	tenantID, didID, err := s.routingScope(r)
	if err != nil {
		writeError(w, err.code, err.msg)
		return
	}
	var in didRoutingRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if code := validateRoutingRule(in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	priority := 100
	if in.Priority != nil {
		priority = *in.Priority
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	created, err2 := s.store.CreateDIDRoutingRule(r.Context(), store.DIDRoutingRule{
		TenantID:       tenantID,
		DIDID:          didID,
		Name:           strings.TrimSpace(in.Name),
		Priority:       priority,
		Enabled:        enabled,
		Timezone:       in.Timezone,
		DaysOfWeek:     in.DaysOfWeek,
		StartMinute:    in.StartMinute,
		EndMinute:      in.EndMinute,
		CallerPrefixes: in.CallerPrefixes,
		Language:       strings.TrimSpace(in.Language),
		TargetBotID:    in.TargetBotID,
		FallbackBotID:  in.FallbackBotID,
	})
	if err2 != nil {
		if errors.Is(err2, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "bot_or_did_not_in_tenant")
			return
		}
		s.logger.Error("create routing rule", "error", err2)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "did_routing.create", "did_routing_rule", created.ID, map[string]any{
		"didId": didID, "name": in.Name,
	})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateDIDRoutingRule(w http.ResponseWriter, r *http.Request) {
	tenantID, didID, err := s.routingScope(r)
	if err != nil {
		writeError(w, err.code, err.msg)
		return
	}
	id := r.PathValue("ruleId")
	var in didRoutingRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if code := validateRoutingRule(in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	current, err2 := s.store.GetDIDRoutingRule(r.Context(), tenantID, id)
	if err2 != nil {
		if errors.Is(err2, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	if current.DIDID != didID {
		writeError(w, http.StatusNotFound, "rule_not_found")
		return
	}
	priority := current.Priority
	if in.Priority != nil {
		priority = *in.Priority
	}
	enabled := current.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	updated, err2 := s.store.UpdateDIDRoutingRule(r.Context(), store.DIDRoutingRule{
		ID:             id,
		TenantID:       tenantID,
		DIDID:          didID,
		Name:           strings.TrimSpace(in.Name),
		Priority:       priority,
		Enabled:        enabled,
		Timezone:       in.Timezone,
		DaysOfWeek:     in.DaysOfWeek,
		StartMinute:    in.StartMinute,
		EndMinute:      in.EndMinute,
		CallerPrefixes: in.CallerPrefixes,
		Language:       strings.TrimSpace(in.Language),
		TargetBotID:    in.TargetBotID,
		FallbackBotID:  in.FallbackBotID,
	})
	if err2 != nil {
		if errors.Is(err2, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "bot_not_in_tenant")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "did_routing.update", "did_routing_rule", id, nil)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteDIDRoutingRule(w http.ResponseWriter, r *http.Request) {
	tenantID, _, err := s.routingScope(r)
	if err != nil {
		writeError(w, err.code, err.msg)
		return
	}
	id := r.PathValue("ruleId")
	if err2 := s.store.DeleteDIDRoutingRule(r.Context(), tenantID, id); err2 != nil {
		if errors.Is(err2, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "did_routing.delete", "did_routing_rule", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// routingScope encuentra el tenant del DID y verifica que el caller
// (platform_admin) tiene acceso. Devuelve (tenant_id, did_id, error).
type httpErr struct {
	code int
	msg  string
}

func (s *Server) routingScope(r *http.Request) (string, string, *httpErr) {
	didID := r.PathValue("didId")
	if didID == "" {
		return "", "", &httpErr{http.StatusBadRequest, "did_required"}
	}
	// Resolvemos tenant del DID directamente — el handler vive bajo
	// requireAuth con role platform_admin (registrado en server.go).
	did, err := s.store.GetDID(r.Context(), didID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", "", &httpErr{http.StatusNotFound, "did_not_found"}
		}
		return "", "", &httpErr{http.StatusInternalServerError, "lookup_failed"}
	}
	if did.TenantID == nil || *did.TenantID == "" {
		return "", "", &httpErr{http.StatusBadRequest, "did_unassigned"}
	}
	return *did.TenantID, didID, nil
}

// validateRoutingRule confía en el CHECK constraint de la BD pero filtra
// payloads obviamente malos antes de llegar a Postgres (mensajes más
// claros para la UI). Lista no exhaustiva — la BD es la última línea.
func validateRoutingRule(in didRoutingRuleInput) string {
	if strings.TrimSpace(in.Name) == "" {
		return "name_required"
	}
	if in.TargetBotID == "" {
		return "target_bot_required"
	}
	if (in.StartMinute == nil) != (in.EndMinute == nil) {
		return "minutes_must_both_be_set_or_none"
	}
	if in.StartMinute != nil {
		if *in.StartMinute < 0 || *in.StartMinute > 1439 || *in.EndMinute < 0 || *in.EndMinute > 1439 {
			return "minutes_out_of_range"
		}
	}
	for _, d := range in.DaysOfWeek {
		if d < 0 || d > 6 {
			return "day_out_of_range"
		}
	}
	if in.Timezone != "" {
		if _, err := time.LoadLocation(in.Timezone); err != nil {
			return "timezone_invalid"
		}
	}
	return ""
}
