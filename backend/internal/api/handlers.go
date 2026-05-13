package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"timbre/backend/internal/ari"
	"timbre/backend/internal/auth"
	"timbre/backend/internal/store"
	"timbre/backend/internal/voiceagent"
)

// --- Auth ---

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expiresAt"`
	User      store.User `json:"user"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email_and_password_required")
		return
	}
	u, err := s.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	tenantID := ""
	if u.TenantID != nil {
		tenantID = *u.TenantID
	}
	token, exp, err := auth.IssueToken(s.cfg.JWTSecret, u.ID, u.Email, u.Role, tenantID, s.cfg.JWTTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_issue_failed")
		return
	}
	_ = s.store.TouchUserLogin(r.Context(), u.ID)
	tenantForAudit := ""
	if u.TenantID != nil {
		tenantForAudit = *u.TenantID
	}
	s.store.WriteAudit(r.Context(), store.AuditEvent{
		TenantID:   tenantForAudit,
		ActorID:    u.ID,
		Action:     "auth.login",
		EntityType: "user",
		EntityID:   u.ID,
		Payload:    map[string]any{"ip": clientIP(r)},
	})
	writeJSON(w, http.StatusOK, loginResponse{Token: token, ExpiresAt: exp, User: u})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       claims.Sub,
		"email":    claims.Email,
		"role":     claims.Role,
		"tenantId": claims.TenantID,
	})
}

// --- Tenant-scoped resources ---

func (s *Server) tenantScope(r *http.Request) (string, error) {
	override := r.URL.Query().Get("tenant")
	return auth.TenantOrOverride(r.Context(), override)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ov, err := s.store.Overview(r.Context(), tenantID)
	if err != nil {
		s.logger.Error("overview", "error", err)
		writeError(w, http.StatusInternalServerError, "overview_failed")
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (s *Server) handleLeads(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	leads, err := s.store.ListLeads(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, leads)
}

func (s *Server) handleCreateLead(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input store.Lead
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.TenantID = tenantID
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Phone) == "" {
		writeError(w, http.StatusBadRequest, "name_and_phone_required")
		return
	}
	if input.Type == "" {
		input.Type = "renter"
	}
	if input.Source == "" {
		input.Source = "portal"
	}
	if input.Consent == "" {
		input.Consent = "manual"
	}
	created, err := s.store.CreateLead(r.Context(), input)
	if err != nil {
		s.logger.Error("create lead", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleProperties(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	props, err := s.store.ListProperties(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, props)
}

func (s *Server) handleBots(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bots, err := s.store.ListBots(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, bots)
}

func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	camps, err := s.store.ListCampaigns(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, camps)
}

func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input store.Campaign
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	input.TenantID = tenantID
	if strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	created, err := s.store.CreateCampaign(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleCalls(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	calls, err := s.store.ListCalls(r.Context(), tenantID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, calls)
}

// --- Test call (ARI originate) ---

type testCallRequest struct {
	Phone    string `json:"phone"`
	LeadName string `json:"leadName"`
	BotID    string `json:"botId"`
}

func (s *Server) handleTestCall(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req testCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.Phone == "" {
		writeError(w, http.StatusBadRequest, "phone_required")
		return
	}

	blocked, err := s.store.IsBlockedPhone(r.Context(), tenantID, req.Phone)
	if err == nil && blocked {
		writeError(w, http.StatusForbidden, "phone_in_do_not_call")
		return
	}

	// Resolve outbound route. If the caller picked a bot, use its assigned DID + trunk.
	// Otherwise fall back to the internal sandbox extension so the plumbing still works.
	endpoint := s.cfg.SIP.TestExtension
	callerID := s.cfg.SIP.CallerID
	routeNote := "internal_sandbox"
	var didID, trunkEndpoint string
	var bot store.Bot

	if req.BotID != "" {
		var err error
		bot, err = s.store.GetBot(r.Context(), tenantID, req.BotID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "bot_not_found")
				return
			}
			s.logger.Error("get bot", "error", err)
			writeError(w, http.StatusInternalServerError, "lookup_failed")
			return
		}
		did, err := s.store.LookupDIDForBot(r.Context(), tenantID, req.BotID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "bot_has_no_did")
				return
			}
			s.logger.Error("lookup did", "error", err)
			writeError(w, http.StatusInternalServerError, "lookup_failed")
			return
		}
		endpoint = "PJSIP/" + req.Phone + "@" + did.AsteriskEndpoint
		callerID = did.Label
		if callerID == "" {
			callerID = "timbre.ai <" + did.E164 + ">"
		}
		didID = did.ID
		trunkEndpoint = did.AsteriskEndpoint
		routeNote = "bot_did"
	}

	call := store.Call{
		TenantID: tenantID,
		Phone:    req.Phone,
		LeadName: defaultStr(req.LeadName, "Test call"),
		Campaign: "Manual test",
		Status:   "queued",
		Outcome:  "pending",
		Summary:  "Manual test call originated from portal (" + routeNote + ").",
	}
	created, err := s.store.CreateCall(r.Context(), call)
	if err != nil {
		s.logger.Error("create test call", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "call.test_originate", "call", created.ID, map[string]any{
		"phone": req.Phone, "botId": req.BotID, "didId": didID, "route": routeNote,
	})

	// Register a voice-agent session if available. Best-effort: failures don't block the call.
	if s.voiceAgent != nil && s.voiceAgent.Enabled() && req.BotID != "" {
		provider := bot.VoiceProvider
		if provider == "" {
			provider = "echo"
		}
		voiceCtx, cancel := contextWithTimeout(r.Context(), 5*time.Second)
		sess, err := s.voiceAgent.CreateSession(voiceCtx, voiceagent.Config{
			CallID:     created.ID,
			TenantID:   tenantID,
			BotID:      req.BotID,
			Provider:   provider,
			Objective:  bot.Objective,
			Guardrails: bot.Guardrails,
			Language:   bot.Language,
			Voice:      bot.Voice,
			LeadName:   req.LeadName,
		})
		cancel()
		if err != nil {
			s.logger.Warn("voice-agent session create", "error", err)
		} else {
			_ = s.store.SetCallVoiceSession(r.Context(), tenantID, created.ID, sess.ID)
			created.VoiceSessionID = sess.ID
			s.audit(r, "voice.session_created", "voice_session", sess.ID, map[string]any{
				"provider": sess.Provider, "callId": created.ID,
			})
		}
	}

	if !s.cfg.ARI.Enabled || s.ari == nil {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"call":     created,
			"endpoint": endpoint,
			"didId":    didID,
			"trunk":    trunkEndpoint,
			"message":  "ARI disabled. Set ASTERISK_ARI_ENABLED=true to originate real channels.",
		})
		return
	}

	originateCtx, cancel := contextWithTimeout(r.Context(), s.cfg.SIP.OriginateTimeout)
	defer cancel()

	ch, err := s.ari.Originate(originateCtx, ari.OriginateRequest{
		Endpoint: endpoint,
		AppArgs:  created.ID + "," + tenantID,
		CallerID: callerID,
		Timeout:  int(s.cfg.SIP.OriginateTimeout.Seconds()),
		Variables: map[string]string{
			"TIMBRE_CALL_ID": created.ID,
			"TIMBRE_TENANT":  tenantID,
			"TIMBRE_BOT":     req.BotID,
			"TIMBRE_DID":     didID,
		},
	})
	if err != nil {
		s.logger.Error("ari originate", "error", err, "call_id", created.ID, "endpoint", endpoint)
		writeError(w, http.StatusBadGateway, "ari_originate_failed")
		return
	}
	if err := s.store.UpdateCallChannel(r.Context(), tenantID, created.ID, ch.ID, "dialing"); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Warn("update call channel", "error", err)
	}
	created.ChannelID = ch.ID
	created.Status = "dialing"
	writeJSON(w, http.StatusAccepted, map[string]any{
		"call":     created,
		"channel":  ch,
		"endpoint": endpoint,
		"didId":    didID,
		"trunk":    trunkEndpoint,
	})
}

var _ = strings.ReplaceAll

// --- Admin ---

func (s *Server) handleTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.store.ListTenants(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, tenants)
}

func (s *Server) handleOperations(w http.ResponseWriter, r *http.Request) {
	trunks, _ := s.store.ListTrunks(r.Context())
	activeTrunks := 0
	for _, t := range trunks {
		if t.Status == "active" {
			activeTrunks++
		}
	}
	voiceProviders := []string{}
	voiceAgentReachable := false
	if s.voiceAgent != nil && s.voiceAgent.Enabled() {
		ctx, cancel := contextWithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if providers, err := s.voiceAgent.Providers(ctx); err == nil {
			voiceProviders = providers
			voiceAgentReachable = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ariEnabled":          s.cfg.ARI.Enabled,
		"ariApp":              s.cfg.ARI.App,
		"sipTestExt":          s.cfg.SIP.TestExtension,
		"trunkCount":          len(trunks),
		"activeTrunks":        activeTrunks,
		"jwtTtlHours":         int(s.cfg.JWTTTL.Hours()),
		"voiceAgentReachable": voiceAgentReachable,
		"voiceProviders":      voiceProviders,
		"version":             "0.1.0",
	})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
