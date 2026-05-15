package api

// Tools / function calling — CRUD por bot.
//
// El editor de bots del frontend lista y edita las tools que ese bot
// expone al LLM. La validación de action_type/action_config se hace
// aquí en el servidor (whitelist estricta) para que un cliente
// malicioso no pueda persistir tipos arbitrarios.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"timbre/backend/internal/store"
)

// allowedActionTypes es el set autorizado que persistimos. Si añades un
// tipo nuevo aquí, añade también su rama en executeToolAction (en el
// handler de invocación) Y en la constraint CHECK de la migration.
var allowedActionTypes = map[string]bool{
	"set_lead_outcome":      true,
	"set_lead_status":       true,
	"schedule_callback":     true,
	"webhook":               true,
	"end_call":              true,
	"transfer_human":        true,
	"search_knowledge_base": true,
}

type botToolInput struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	ParametersSchema map[string]any `json:"parametersSchema"`
	ActionType       string         `json:"actionType"`
	ActionConfig     map[string]any `json:"actionConfig"`
	Enabled          *bool          `json:"enabled,omitempty"`
}

func (s *Server) handleListBotTools(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	tools, err := s.store.ListBotTools(r.Context(), tenantID, botID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, tools)
}

func (s *Server) handleCreateBotTool(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")

	var input botToolInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if code := validateBotToolInput(input); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	created, err := s.store.CreateBotTool(r.Context(), store.BotTool{
		TenantID:         tenantID,
		BotID:            botID,
		Name:             input.Name,
		Description:      input.Description,
		ParametersSchema: input.ParametersSchema,
		ActionType:       input.ActionType,
		ActionConfig:     input.ActionConfig,
		Enabled:          enabled,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "bot_not_in_tenant")
			return
		}
		// Nombre duplicado (UNIQUE bot_id,name) — error útil para la UI.
		if strings.Contains(err.Error(), "bot_tools_name_unique") {
			writeError(w, http.StatusConflict, "tool_name_taken")
			return
		}
		s.logger.Error("create bot tool", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "bot_tool.create", "bot_tool", created.ID, map[string]any{
		"botId": botID, "name": input.Name, "actionType": input.ActionType,
	})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateBotTool(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("toolId")

	var input botToolInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	// Action type es inmutable en update (ver comentario en store).
	// Cargamos el actual para preservarlo.
	current, err := s.store.GetBotTool(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tool_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	input.ActionType = current.ActionType
	if code := validateBotToolInput(input); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	enabled := current.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	updated, err := s.store.UpdateBotTool(r.Context(), store.BotTool{
		ID:               id,
		TenantID:         tenantID,
		Name:             input.Name,
		Description:      input.Description,
		ParametersSchema: input.ParametersSchema,
		ActionConfig:     input.ActionConfig,
		Enabled:          enabled,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tool_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "bot_tool.update", "bot_tool", id, map[string]any{"name": input.Name})
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteBotTool(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("toolId")
	if err := s.store.DeleteBotTool(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tool_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "bot_tool.delete", "bot_tool", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// validateBotToolInput devuelve "" si es válido, o un código de error
// estable que el frontend usa para mostrar el mensaje correcto.
func validateBotToolInput(in botToolInput) string {
	if strings.TrimSpace(in.Name) == "" {
		return "name_required"
	}
	// Mismas reglas que function-name de OpenAI/Deepgram: 1-64 chars,
	// [a-zA-Z0-9_-]. Lo aplicamos en el servidor para que un nombre
	// inválido no rompa la sesión más tarde con el provider.
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
	// action_config sanity por tipo. Lo justo para no aceptar configs
	// que romperán la ejecución silenciosamente.
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
