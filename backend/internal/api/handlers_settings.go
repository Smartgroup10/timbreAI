package api

import (
	"encoding/json"
	"net/http"

	"atrium-calls/backend/internal/store"
)

func (s *Server) handleTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ts, err := s.store.GetTenantSettings(r.Context(), tenantID)
	if err != nil {
		s.logger.Error("tenant settings get", "error", err)
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, ts)
}

func (s *Server) handleUpdateTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var p store.TenantSettingsPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	ts, err := s.store.UpdateTenantSettings(r.Context(), tenantID, p)
	if err != nil {
		s.logger.Error("tenant settings update", "error", err)
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	payload := map[string]any{}
	if p.Timezone != nil {
		payload["timezone"] = *p.Timezone
	}
	if p.CallerIDDefault != nil {
		payload["callerIdDefault"] = *p.CallerIDDefault
	}
	if p.DailyCallCap != nil {
		payload["dailyCallCap"] = *p.DailyCallCap
	}
	s.audit(r, "tenant_settings.update", "tenant_settings", tenantID, payload)
	writeJSON(w, http.StatusOK, ts)
}
