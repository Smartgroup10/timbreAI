package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"timbre/backend/internal/auth"
	"timbre/backend/internal/store"
)

// --- Lead mutations ---

func (s *Server) handleUpdateLead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var p store.LeadPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	lead, err := s.store.UpdateLead(r.Context(), tenantID, id, p)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "lead_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "lead.update", "lead", id, leadDiff(p))
	s.emitRealtime(tenantID, "lead.updated", map[string]any{"leadId": id, "status": lead.Status})
	writeJSON(w, http.StatusOK, lead)
}

func (s *Server) handleDeleteLead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteLead(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "lead_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "lead.delete", "lead", id, nil)
	s.emitRealtime(tenantID, "lead.deleted", map[string]any{"leadId": id})
	w.WriteHeader(http.StatusNoContent)
}

func leadDiff(p store.LeadPatch) map[string]any {
	m := map[string]any{}
	if p.Status != nil {
		m["status"] = *p.Status
	}
	if p.Consent != nil {
		m["consent"] = *p.Consent
	}
	return m
}

// --- Bot mutations ---

func (s *Server) handleCreateBot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input store.Bot
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.TenantID = tenantID
	if strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	bot, err := s.store.CreateBot(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "bot.create", "bot", bot.ID, map[string]any{"name": bot.Name})
	writeJSON(w, http.StatusCreated, bot)
}

func (s *Server) handleUpdateBot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var p store.BotPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	bot, err := s.store.UpdateBot(r.Context(), tenantID, id, p)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bot_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "bot.update", "bot", id, nil)
	writeJSON(w, http.StatusOK, bot)
}

func (s *Server) handleDeleteBot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteBot(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bot_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "bot.delete", "bot", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// --- Campaign mutations ---

func (s *Server) handleUpdateCampaign(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var p store.CampaignPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	c, err := s.store.UpdateCampaign(r.Context(), tenantID, id, p)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	payload := map[string]any{}
	if p.Status != nil {
		payload["status"] = *p.Status
	}
	s.audit(r, "campaign.update", "campaign", id, payload)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteCampaign(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteCampaign(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "campaign.delete", "campaign", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// --- Property CRUD ---

func (s *Server) handleCreateProperty(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input store.Property
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.TenantID = tenantID
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Address) == "" {
		writeError(w, http.StatusBadRequest, "name_and_address_required")
		return
	}
	p, err := s.store.CreateProperty(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "property.create", "property", p.ID, map[string]any{"name": p.Name})
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdateProperty(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var p store.PropertyPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := s.store.UpdateProperty(r.Context(), tenantID, id, p); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "property_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "property.update", "property", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteProperty(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteProperty(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "property_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "property.delete", "property", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// --- Password change ---

type changePasswordRequest struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if len(req.New) < 8 {
		writeError(w, http.StatusBadRequest, "password_too_short")
		return
	}
	user, err := s.store.GetUser(r.Context(), claims.Sub)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user_not_found")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Current) {
		writeError(w, http.StatusUnauthorized, "invalid_current_password")
		return
	}
	hash, err := auth.HashPassword(req.New)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash_failed")
		return
	}
	if err := s.store.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "user.password_change", "user", user.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}
