package api

import "net/http"

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Tenant timezone drives the daily buckets.
	tz := "UTC"
	if ts, err := s.store.GetTenantSettings(r.Context(), tenantID); err == nil && ts.Timezone != "" {
		tz = ts.Timezone
	}
	report, err := s.store.BuildAnalytics(r.Context(), tenantID, tz)
	if err != nil {
		s.logger.Error("analytics", "error", err)
		writeError(w, http.StatusInternalServerError, "analytics_failed")
		return
	}
	writeJSON(w, http.StatusOK, report)
}
