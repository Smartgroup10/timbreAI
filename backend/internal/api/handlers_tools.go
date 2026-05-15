package api

// Tools / function calling — CRUD biblioteca por tenant + assignments
// por bot.
//
// Endpoints:
//   GET    /api/tools                                — biblioteca
//   POST   /api/tools                                — crear
//   PATCH  /api/tools/{id}                           — editar (action_type es inmutable)
//   DELETE /api/tools/{id}                           — eliminar (cascade asignaciones)
//   GET    /api/bots/{id}/tools                      — assignments view del bot
//   PUT    /api/bots/{id}/tools/{toolId}             — asignar / actualizar enabled
//   DELETE /api/bots/{id}/tools/{toolId}             — desasignar

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"timbre/backend/internal/store"
)

// allowedActionTypes whitelist. Whitelist OS + DB CHECK constraint —
// defensa en profundidad ante CHECK olvidado en migración.
var allowedActionTypes = map[string]bool{
	"set_lead_outcome":             true,
	"set_lead_status":              true,
	"schedule_callback":            true,
	"webhook":                      true,
	"end_call":                     true,
	"transfer_human":               true,
	"search_knowledge_base":        true,
	"calendar_check_availability":  true,
	"calendar_schedule_meeting":    true,
	"calendar_list_my_meetings":    true,
	"calendar_cancel_meeting":      true,
	"calendar_reschedule_meeting":  true,
}

type toolInput struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	ParametersSchema map[string]any `json:"parametersSchema"`
	ActionType       string         `json:"actionType"`
	ActionConfig     map[string]any `json:"actionConfig"`
	Enabled          *bool          `json:"enabled,omitempty"`
}

// ─── CRUD biblioteca ────────────────────────────────────────────────────

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tools, err := s.store.ListTools(r.Context(), tenantID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, tools)
}

func (s *Server) handleCreateTool(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var in toolInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if code := validateToolInput(in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	created, err := s.store.CreateTool(r.Context(), store.Tool{
		TenantID:         tenantID,
		Name:             in.Name,
		Description:      in.Description,
		ParametersSchema: in.ParametersSchema,
		ActionType:       in.ActionType,
		ActionConfig:     in.ActionConfig,
		Enabled:          enabled,
	})
	if err != nil {
		if strings.Contains(err.Error(), "tools_tenant_name_unique") {
			writeError(w, http.StatusConflict, "tool_name_taken")
			return
		}
		s.logger.Error("create tool", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "tool.create", "tool", created.ID, map[string]any{"name": in.Name, "actionType": in.ActionType})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateTool(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var in toolInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	current, err := s.store.GetTool(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tool_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	// action_type es inmutable — preservamos el actual antes de validar.
	in.ActionType = current.ActionType
	if code := validateToolInput(in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	enabled := current.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	updated, err := s.store.UpdateTool(r.Context(), store.Tool{
		ID:               id,
		TenantID:         tenantID,
		Name:             in.Name,
		Description:      in.Description,
		ParametersSchema: in.ParametersSchema,
		ActionType:       current.ActionType,
		ActionConfig:     in.ActionConfig,
		Enabled:          enabled,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tool_not_found")
			return
		}
		if strings.Contains(err.Error(), "tools_tenant_name_unique") {
			writeError(w, http.StatusConflict, "tool_name_taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "tool.update", "tool", id, map[string]any{"name": in.Name})
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteTool(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteTool(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tool_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "tool.delete", "tool", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ─── Assignments ────────────────────────────────────────────────────────

func (s *Server) handleListBotToolAssignments(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	views, err := s.store.ListBotToolViews(r.Context(), tenantID, botID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, views)
}

type assignToolInput struct {
	Enabled bool `json:"enabled"`
}

// PUT idempotente: si la tool no estaba asignada al bot, la asigna;
// si ya estaba, actualiza el flag enabled.
func (s *Server) handleAssignToolToBot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	toolID := r.PathValue("toolId")
	in := assignToolInput{Enabled: true}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	if err := s.store.AssignToolToBot(r.Context(), tenantID, botID, toolID, in.Enabled); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bot_or_tool_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "assign_failed")
		return
	}
	s.audit(r, "tool.assign", "bot", botID, map[string]any{"toolId": toolID, "enabled": in.Enabled})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnassignToolFromBot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	toolID := r.PathValue("toolId")
	if err := s.store.UnassignToolFromBot(r.Context(), tenantID, botID, toolID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "assignment_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "unassign_failed")
		return
	}
	s.audit(r, "tool.unassign", "bot", botID, map[string]any{"toolId": toolID})
	w.WriteHeader(http.StatusNoContent)
}

func validateToolInput(in toolInput) string {
	if strings.TrimSpace(in.Name) == "" {
		return "name_required"
	}
	if len(in.Name) > 64 {
		return "name_too_long"
	}
	for _, r := range in.Name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			return "name_invalid_chars"
		}
	}
	if strings.TrimSpace(in.Description) == "" {
		return "description_required"
	}
	if !allowedActionTypes[in.ActionType] {
		return "action_type_invalid"
	}
	switch in.ActionType {
	case "webhook":
		url, _ := in.ActionConfig["url"].(string)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return "webhook_url_invalid"
		}
	case "set_lead_outcome", "set_lead_status":
		v, _ := in.ActionConfig["value"].(string)
		if strings.TrimSpace(v) == "" {
			return "value_required"
		}
	}
	return ""
}
