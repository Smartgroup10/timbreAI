package api

import (
	"log/slog"
	"net/http"
	"time"

	"timbre/backend/internal/ari"
	"timbre/backend/internal/config"
	"timbre/backend/internal/pricing"
	"timbre/backend/internal/storage"
	"timbre/backend/internal/store"
	"timbre/backend/internal/voiceagent"
)

type Server struct {
	cfg        config.Config
	store      *store.Store
	ari        *ari.Client
	voiceAgent *voiceagent.Client
	storage    *storage.Client
	pricing    *pricing.Table
	logger     *slog.Logger
	loginRate  *rateLimiter
}

func New(cfg config.Config, st *store.Store, ariClient *ari.Client, va *voiceagent.Client, storeClient *storage.Client, logger *slog.Logger) *Server {
	return &Server{
		cfg:        cfg,
		store:      st,
		ari:        ariClient,
		voiceAgent: va,
		storage:    storeClient,
		pricing:    pricing.NewTable(),
		logger:     logger,
		// 1 token per second, burst of 10 => allows brief bursts but blocks brute-force.
		loginRate: newRateLimiter(time.Second, 10),
	}
}

// withCost rellena CostCents y devuelve la lista. Helper para inyectar el
// coste estimado en cada call al serializar — el campo no se persiste.
func (s *Server) withCost(calls []store.Call) []store.Call {
	for i := range calls {
		calls[i].CostCents = s.pricing.Cost(calls[i].Provider, calls[i].DurationSec)
	}
	return calls
}

// withCostOne idem para un único call.
func (s *Server) withCostOne(c store.Call) store.Call {
	c.CostCents = s.pricing.Cost(c.Provider, c.DurationSec)
	return c
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public (rate-limited on login to slow brute-force).
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /api/auth/login", s.limit(s.loginRate, s.handleLogin))
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handleChangePassword))

	// Tenant-scoped (any authenticated user)
	mux.HandleFunc("GET /api/overview", s.requireAuth(s.handleOverview))

	mux.HandleFunc("GET /api/leads", s.requireAuth(s.handleLeads))
	mux.HandleFunc("POST /api/leads", s.requireAuth(s.handleCreateLead))
	mux.HandleFunc("POST /api/leads/import", s.requireAuth(s.handleImportLeads))
	mux.HandleFunc("GET /api/leads/{id}", s.requireAuth(s.handleGetLead))
	mux.HandleFunc("GET /api/leads/{id}/calls", s.requireAuth(s.handleLeadCalls))
	mux.HandleFunc("PATCH /api/leads/{id}", s.requireAuth(s.handleUpdateLead))
	mux.HandleFunc("DELETE /api/leads/{id}", s.requireAuth(s.handleDeleteLead))

	mux.HandleFunc("GET /api/properties", s.requireAuth(s.handleProperties))
	mux.HandleFunc("POST /api/properties", s.requireAuth(s.handleCreateProperty))
	mux.HandleFunc("PATCH /api/properties/{id}", s.requireAuth(s.handleUpdateProperty))
	mux.HandleFunc("DELETE /api/properties/{id}", s.requireAuth(s.handleDeleteProperty))

	mux.HandleFunc("GET /api/bots", s.requireAuth(s.handleBots))
	mux.HandleFunc("POST /api/bots", s.requireAuth(s.handleCreateBot))
	mux.HandleFunc("PATCH /api/bots/{id}", s.requireAuth(s.handleUpdateBot))
	mux.HandleFunc("DELETE /api/bots/{id}", s.requireAuth(s.handleDeleteBot))
	mux.HandleFunc("POST /api/bots/{id}/did", s.requireAuth(s.handleAssignBotDID))
	// Tools (function calling) por bot.
	mux.HandleFunc("GET /api/bots/{id}/tools", s.requireAuth(s.handleListBotTools))
	mux.HandleFunc("POST /api/bots/{id}/tools", s.requireAuth(s.handleCreateBotTool))
	mux.HandleFunc("PATCH /api/bots/{id}/tools/{toolId}", s.requireAuth(s.handleUpdateBotTool))
	mux.HandleFunc("DELETE /api/bots/{id}/tools/{toolId}", s.requireAuth(s.handleDeleteBotTool))

	mux.HandleFunc("GET /api/dids", s.requireAuth(s.handleTenantDIDs))

	mux.HandleFunc("GET /api/campaigns", s.requireAuth(s.handleCampaigns))
	mux.HandleFunc("POST /api/campaigns", s.requireAuth(s.handleCreateCampaign))
	mux.HandleFunc("PATCH /api/campaigns/{id}", s.requireAuth(s.handleUpdateCampaign))
	mux.HandleFunc("DELETE /api/campaigns/{id}", s.requireAuth(s.handleDeleteCampaign))
	mux.HandleFunc("GET /api/campaigns/{id}/leads", s.requireAuth(s.handleListCampaignLeads))
	mux.HandleFunc("POST /api/campaigns/{id}/leads", s.requireAuth(s.handleAddCampaignLeads))
	mux.HandleFunc("DELETE /api/campaigns/{id}/leads/{leadId}", s.requireAuth(s.handleRemoveCampaignLead))

	mux.HandleFunc("GET /api/calls", s.requireAuth(s.handleCalls))
	mux.HandleFunc("GET /api/calls/{id}", s.requireAuth(s.handleGetCall))
	mux.HandleFunc("GET /api/calls/{id}/transcripts", s.requireAuth(s.handleCallTranscripts))
	mux.HandleFunc("POST /api/calls/test", s.requireAuth(s.handleTestCall))

	mux.HandleFunc("GET /api/dnc", s.requireAuth(s.handleDNCList))
	mux.HandleFunc("POST /api/dnc", s.requireAuth(s.handleDNCAdd))
	mux.HandleFunc("DELETE /api/dnc/{id}", s.requireAuth(s.handleDNCDelete))

	mux.HandleFunc("GET /api/audit", s.requireAuth(s.handleTenantAuditList))

	mux.HandleFunc("GET /api/tenant/settings", s.requireAuth(s.handleTenantSettings))
	mux.HandleFunc("PATCH /api/tenant/settings", s.requireAuth(s.handleUpdateTenantSettings))

	// Per-tenant voice provider credentials (API keys + models). Masked on read.
	mux.HandleFunc("GET /api/tenant/voice-credentials", s.requireTenantAdmin(s.handleGetVoiceCredentials))
	mux.HandleFunc("PATCH /api/tenant/voice-credentials", s.requireTenantAdmin(s.handleUpdateVoiceCredentials))
	mux.HandleFunc("POST /api/tenant/voice-credentials/test", s.requireTenantAdmin(s.handleTestVoiceCredentials))
	mux.HandleFunc("GET /api/voice-catalog", s.requireAuth(s.handleGetVoiceCatalog))

	// Per-tenant user management (tenant_admin or platform_admin).
	mux.HandleFunc("GET /api/tenant/users", s.requireTenantAdmin(s.handleListTenantUsers))
	mux.HandleFunc("POST /api/tenant/users", s.requireTenantAdmin(s.handleInviteTenantUser))
	mux.HandleFunc("PATCH /api/tenant/users/{id}", s.requireTenantAdmin(s.handleUpdateTenantUser))
	mux.HandleFunc("DELETE /api/tenant/users/{id}", s.requireTenantAdmin(s.handleDeleteTenantUser))

	// Internal endpoints: only the voice-agent calls these, protected by shared secret.
	mux.HandleFunc("POST /api/internal/voice/transcripts", s.requireInternalSecret(s.handleInternalTranscript))
	mux.HandleFunc("POST /api/internal/voice/recordings", s.requireInternalSecret(s.handleInternalRecording))
	mux.HandleFunc("POST /api/internal/voice/tool-invoke", s.requireInternalSecret(s.handleInternalToolInvoke))

	mux.HandleFunc("GET /api/analytics", s.requireAuth(s.handleAnalytics))
	mux.HandleFunc("GET /api/pricing", s.requireAuth(s.handlePricing))

	// Platform-admin only
	mux.HandleFunc("GET /api/admin/tenants", s.requireRole("platform_admin", s.handleTenants))
	mux.HandleFunc("POST /api/admin/tenants", s.requireRole("platform_admin", s.handleAdminCreateTenant))
	mux.HandleFunc("PATCH /api/admin/tenants/{id}", s.requireRole("platform_admin", s.handleAdminUpdateTenant))
	mux.HandleFunc("GET /api/admin/operations", s.requireRole("platform_admin", s.handleOperations))
	mux.HandleFunc("GET /api/admin/audit", s.requireRole("platform_admin", s.handleAuditList))
	mux.HandleFunc("GET /api/admin/trunks", s.requireRole("platform_admin", s.handleAdminListTrunks))
	mux.HandleFunc("GET /api/admin/trunks/status", s.requireRole("platform_admin", s.handleAdminTrunkStatus))
	mux.HandleFunc("POST /api/admin/trunks", s.requireRole("platform_admin", s.handleAdminCreateTrunk))
	mux.HandleFunc("PATCH /api/admin/trunks/{id}", s.requireRole("platform_admin", s.handleAdminUpdateTrunk))
	mux.HandleFunc("DELETE /api/admin/trunks/{id}", s.requireRole("platform_admin", s.handleAdminDeleteTrunk))
	mux.HandleFunc("GET /api/admin/dids", s.requireRole("platform_admin", s.handleAdminListDIDs))
	mux.HandleFunc("POST /api/admin/dids", s.requireRole("platform_admin", s.handleAdminCreateDID))
	mux.HandleFunc("PATCH /api/admin/dids/{id}", s.requireRole("platform_admin", s.handleAdminUpdateDID))
	mux.HandleFunc("PATCH /api/admin/dids/{id}/assign", s.requireRole("platform_admin", s.handleAdminAssignDID))
	mux.HandleFunc("DELETE /api/admin/dids/{id}", s.requireRole("platform_admin", s.handleAdminDeleteDID))

	return s.cors(s.logRequests(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"version":    "0.1.0",
		"ariEnabled": s.cfg.ARI.Enabled,
		"time":       time.Now().UTC().Format(time.RFC3339),
	})
}
