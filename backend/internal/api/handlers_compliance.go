package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"timbre/backend/internal/auth"
	"timbre/backend/internal/store"
)

// --- Do Not Call ---

func (s *Server) handleDNCList(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := s.store.ListDoNotCall(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleDNCAdd(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input store.DoNotCallEntry
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.TenantID = tenantID
	if strings.TrimSpace(input.Phone) == "" {
		writeError(w, http.StatusBadRequest, "phone_required")
		return
	}
	created, err := s.store.AddDoNotCall(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "dnc.add", "do_not_call", created.ID, map[string]any{"phone": input.Phone, "reason": input.Reason})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDNCDelete(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.RemoveDoNotCall(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "dnc.remove", "do_not_call", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// --- Audit log ---

// audit is a tiny convenience wrapper for handlers to log an action with the caller's identity.
func (s *Server) audit(r *http.Request, action, entityType, entityID string, payload map[string]any) {
	claims, _ := auth.FromContext(r.Context())
	s.store.WriteAudit(r.Context(), store.AuditEvent{
		TenantID:   claims.TenantID,
		ActorID:    claims.Sub,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Payload:    payload,
	})
}

func (s *Server) handleAuditList(w http.ResponseWriter, r *http.Request) {
	tenantFilter := r.URL.Query().Get("tenant")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := s.store.ListAudit(r.Context(), tenantFilter, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// Tenant-scoped audit view (only their own).
func (s *Server) handleTenantAuditList(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := s.store.ListAudit(r.Context(), tenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
