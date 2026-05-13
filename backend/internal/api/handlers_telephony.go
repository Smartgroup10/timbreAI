package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"timbre/backend/internal/auth"
	"timbre/backend/internal/store"
)

// --- Admin: trunks ---

func (s *Server) handleAdminListTrunks(w http.ResponseWriter, r *http.Request) {
	trunks, err := s.store.ListTrunks(r.Context())
	if err != nil {
		s.logger.Error("list trunks", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, trunks)
}

func (s *Server) handleAdminCreateTrunk(w http.ResponseWriter, r *http.Request) {
	var input store.SIPTrunk
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.AsteriskEndpoint = strings.TrimSpace(input.AsteriskEndpoint)
	if input.Name == "" || input.AsteriskEndpoint == "" {
		writeError(w, http.StatusBadRequest, "name_and_endpoint_required")
		return
	}
	created, err := s.store.CreateTrunk(r.Context(), input)
	if err != nil {
		s.logger.Error("create trunk", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleAdminUpdateTrunk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id_required")
		return
	}
	var input store.SIPTrunk
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.ID = id
	if err := s.store.UpdateTrunk(r.Context(), input); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "trunk_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminDeleteTrunk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteTrunk(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "trunk_not_found")
			return
		}
		s.logger.Warn("delete trunk", "error", err)
		writeError(w, http.StatusConflict, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminTrunkStatus consulta Asterisk vía ARI por el estado real de cada
// endpoint PJSIP. Devuelve un mapa endpoint -> {state, channels} que el portal
// admin pinta junto a los trunks en BD.
func (s *Server) handleAdminTrunkStatus(w http.ResponseWriter, r *http.Request) {
	if s.ari == nil {
		// ARI desactivado: devolvemos vacío en vez de error para que la UI siga renderizando.
		writeJSON(w, http.StatusOK, map[string]any{"endpoints": []any{}, "ariEnabled": false})
		return
	}
	eps, err := s.ari.ListEndpoints(r.Context(), "PJSIP")
	if err != nil {
		s.logger.Warn("ari list endpoints", "error", err)
		writeError(w, http.StatusBadGateway, "ari_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"endpoints": eps, "ariEnabled": true})
}

// --- Admin: DIDs ---

func (s *Server) handleAdminListDIDs(w http.ResponseWriter, r *http.Request) {
	tenantFilter := r.URL.Query().Get("tenant") // optional, narrows the global pool
	dids, err := s.store.ListDIDs(r.Context(), tenantFilter)
	if err != nil {
		s.logger.Error("list dids", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, dids)
}

type adminCreateDIDInput struct {
	TrunkID  string  `json:"trunkId"`
	TenantID *string `json:"tenantId,omitempty"`
	E164     string  `json:"e164"`
	Label    string  `json:"label"`
	Status   string  `json:"status"`
}

func (s *Server) handleAdminCreateDID(w http.ResponseWriter, r *http.Request) {
	var input adminCreateDIDInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.TrunkID = strings.TrimSpace(input.TrunkID)
	input.E164 = strings.TrimSpace(input.E164)
	if input.TrunkID == "" || input.E164 == "" {
		writeError(w, http.StatusBadRequest, "trunk_and_e164_required")
		return
	}
	created, err := s.store.CreateDID(r.Context(), store.DID{
		TrunkID:  input.TrunkID,
		TenantID: input.TenantID,
		E164:     input.E164,
		Label:    input.Label,
		Status:   input.Status,
	})
	if err != nil {
		s.logger.Error("create did", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

type adminAssignDIDInput struct {
	TenantID *string `json:"tenantId"`
}

func (s *Server) handleAdminAssignDID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input adminAssignDIDInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := s.store.AssignDIDToTenant(r.Context(), id, input.TenantID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "did_not_found")
			return
		}
		s.logger.Error("assign did", "error", err)
		writeError(w, http.StatusInternalServerError, "assign_failed")
		return
	}
	target := ""
	if input.TenantID != nil {
		target = *input.TenantID
	}
	s.audit(r, "did.assign_tenant", "did", id, map[string]any{"tenantId": target})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminDeleteDID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteDID(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "did_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Tenant: read assigned DIDs, assign DID to bot ---

func (s *Server) handleTenantDIDs(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dids, err := s.store.ListDIDs(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, dids)
}

type assignBotDIDInput struct {
	DIDID *string `json:"didId"`
}

func (s *Server) handleAssignBotDID(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	var input assignBotDIDInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := s.store.AssignBotDID(r.Context(), tenantID, botID, input.DIDID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bot_or_did_not_found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bot, err := s.store.GetBot(r.Context(), tenantID, botID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	didVal := ""
	if input.DIDID != nil {
		didVal = *input.DIDID
	}
	s.audit(r, "bot.assign_did", "bot", botID, map[string]any{"didId": didVal})
	writeJSON(w, http.StatusOK, bot)
}

var _ = auth.RolePlatformAdmin
