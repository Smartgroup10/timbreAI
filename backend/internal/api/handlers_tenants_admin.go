package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"atrium-calls/backend/internal/store"
)

type createTenantInput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Plan   string `json:"plan"`
	Status string `json:"status"`
}

var tenantIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

func (s *Server) handleAdminCreateTenant(w http.ResponseWriter, r *http.Request) {
	var input createTenantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.ID = strings.ToLower(strings.TrimSpace(input.ID))
	input.Name = strings.TrimSpace(input.Name)
	if !tenantIDPattern.MatchString(input.ID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant_id")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	if input.Plan == "" {
		input.Plan = "starter"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if err := s.store.CreateTenant(r.Context(), input.ID, input.Name, input.Plan, input.Status); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "tenant_already_exists")
			return
		}
		s.logger.Error("create tenant", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "tenant.create", "tenant", input.ID, map[string]any{"name": input.Name, "plan": input.Plan})
	tenant, _ := s.store.GetTenant(r.Context(), input.ID)
	writeJSON(w, http.StatusCreated, tenant)
}

type patchTenantInput struct {
	Name   *string `json:"name"`
	Plan   *string `json:"plan"`
	Status *string `json:"status"`
}

func (s *Server) handleAdminUpdateTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input patchTenantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := s.store.UpdateTenant(r.Context(), id, input.Name, input.Plan, input.Status); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tenant_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	payload := map[string]any{}
	if input.Name != nil {
		payload["name"] = *input.Name
	}
	if input.Plan != nil {
		payload["plan"] = *input.Plan
	}
	if input.Status != nil {
		payload["status"] = *input.Status
	}
	s.audit(r, "tenant.update", "tenant", id, payload)
	w.WriteHeader(http.StatusNoContent)
}
