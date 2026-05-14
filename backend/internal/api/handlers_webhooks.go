package api

// Webhooks salientes — CRUD por tenant + log de entregas.
//
// El secret se devuelve solo en POST (create) y POST /regenerate.
// El GET nunca lo devuelve para que un atacante con read-only no pueda
// firmar peticiones falsas.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"timbre/backend/internal/outwebhook"
	"timbre/backend/internal/store"
)

type webhookEndpointInput struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Active *bool    `json:"active,omitempty"`
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	endpoints, err := s.store.ListWebhookEndpoints(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, endpoints)
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var in webhookEndpointInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if code := validateWebhookInput(in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	created, err := s.store.CreateWebhookEndpoint(r.Context(), store.WebhookEndpoint{
		TenantID: tenantID,
		Name:     in.Name,
		URL:      in.URL,
		Events:   in.Events,
		Active:   active,
	})
	if err != nil {
		s.logger.Error("create webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "webhook.create", "webhook", created.ID, map[string]any{
		"name": in.Name, "url": in.URL, "events": in.Events,
	})
	// Devolvemos el secret en la respuesta — única vez que viaja por el cable.
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var in webhookEndpointInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if code := validateWebhookInput(in); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	current, err := s.store.GetWebhookEndpoint(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "webhook_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	active := current.Active
	if in.Active != nil {
		active = *in.Active
	}
	updated, err := s.store.UpdateWebhookEndpoint(r.Context(), store.WebhookEndpoint{
		ID:       id,
		TenantID: tenantID,
		Name:     in.Name,
		URL:      in.URL,
		Events:   in.Events,
		Active:   active,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	s.audit(r, "webhook.update", "webhook", id, map[string]any{"name": in.Name})
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteWebhookEndpoint(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "webhook_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "webhook.delete", "webhook", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// handleRegenerateWebhookSecret rota el secret. Devuelve el nuevo solo
// en la respuesta — el operador tiene que actualizarlo en el receptor.
func (s *Server) handleRegenerateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	secret, err := s.store.RegenerateWebhookSecret(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "webhook_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "regenerate_failed")
		return
	}
	s.audit(r, "webhook.regenerate_secret", "webhook", id, nil)
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret})
}

func (s *Server) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	deliveries, err := s.store.ListWebhookDeliveries(r.Context(), tenantID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, deliveries)
}

// handleWebhookEvents devuelve la lista de event types soportados — la
// UI lo usa para pintar un multi-select con etiquetas traducidas.
func (s *Server) handleWebhookEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"events": outwebhook.AllEvents})
}

// OnCallFinished se inyecta al ARI handler. Cuando una llamada termina,
// despachamos call.completed (y call.qualified si outcome=qualified) a
// los webhooks suscritos del tenant.
//
// El callID es único globalmente (newID lo prefija con "call"), así que
// usamos GetCallByID que ignora tenant_id — el callback no lo tiene a
// mano y querer pasarlo desde el ARI handler complica la firma.
func (s *Server) OnCallFinished(ctx context.Context, callID string) {
	if s.webhooks == nil {
		return
	}
	c, err := s.store.GetCallByID(ctx, callID)
	if err != nil {
		s.logger.Warn("OnCallFinished: lookup", "callId", callID, "error", err)
		return
	}
	payload := map[string]any{
		"call": map[string]any{
			"id":           c.ID,
			"phone":        c.Phone,
			"leadName":     c.LeadName,
			"leadId":       c.LeadID,
			"campaign":     c.Campaign,
			"campaignId":   c.CampaignID,
			"status":       c.Status,
			"outcome":      c.Outcome,
			"durationSec":  c.DurationSec,
			"provider":     c.Provider,
			"costCents":    s.pricing.Cost(c.Provider, c.DurationSec),
			"startedAt":    c.StartedAt,
			"endedAt":      c.EndedAt,
			"summary":      c.Summary,
			"recordingUrl": c.RecordingURL,
		},
	}
	s.webhooks.Dispatch(outwebhook.Event{
		TenantID: c.TenantID,
		Type:     outwebhook.EventCallCompleted,
		Payload:  payload,
	})
	if c.Outcome == "qualified" {
		s.webhooks.Dispatch(outwebhook.Event{
			TenantID: c.TenantID,
			Type:     outwebhook.EventCallQualified,
			Payload:  payload,
		})
	}
}

func validateWebhookInput(in webhookEndpointInput) string {
	if strings.TrimSpace(in.Name) == "" {
		return "name_required"
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return "url_invalid"
	}
	if len(in.Events) == 0 {
		return "events_required"
	}
	allowed := map[string]bool{}
	for _, e := range outwebhook.AllEvents {
		allowed[e] = true
	}
	for _, e := range in.Events {
		if !allowed[e] {
			return "event_unknown"
		}
	}
	return ""
}
