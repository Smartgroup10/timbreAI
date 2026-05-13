package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"timbre/backend/internal/store"
)

func (s *Server) handleListCampaignLeads(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	leads, err := s.store.ListCampaignLeads(r.Context(), tenantID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, leads)
}

type addCampaignLeadsInput struct {
	LeadIDs []string `json:"leadIds"`
}

func (s *Server) handleAddCampaignLeads(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var input addCampaignLeadsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if len(input.LeadIDs) == 0 {
		writeError(w, http.StatusBadRequest, "lead_ids_required")
		return
	}
	created, total, err := s.store.AddLeadsToCampaign(r.Context(), tenantID, id, input.LeadIDs)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign_not_found")
			return
		}
		s.logger.Error("add campaign leads", "error", err)
		writeError(w, http.StatusInternalServerError, "add_failed")
		return
	}
	s.audit(r, "campaign.leads_added", "campaign", id, map[string]any{"added": created, "total": total})
	writeJSON(w, http.StatusOK, map[string]int{"created": created, "total": total})
}

func (s *Server) handleRemoveCampaignLead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	leadID := r.PathValue("leadId")
	if err := s.store.RemoveLeadFromCampaign(r.Context(), tenantID, id, leadID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "campaign.lead_removed", "campaign", id, map[string]any{"leadId": leadID})
	w.WriteHeader(http.StatusNoContent)
}
