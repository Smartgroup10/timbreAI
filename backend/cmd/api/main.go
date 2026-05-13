package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type tenant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Plan      string `json:"plan"`
	CreatedAt string `json:"createdAt"`
}

type lead struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenantId"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Source       string `json:"source"`
	Consent      string `json:"consent"`
	LastActivity string `json:"lastActivity"`
}

type property struct {
	ID           string   `json:"id"`
	TenantID     string   `json:"tenantId"`
	Name         string   `json:"name"`
	Address      string   `json:"address"`
	Price        string   `json:"price"`
	Availability string   `json:"availability"`
	Requirements []string `json:"requirements"`
	FAQs         []string `json:"faqs"`
}

type bot struct {
	ID         string   `json:"id"`
	TenantID   string   `json:"tenantId"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Language   string   `json:"language"`
	Voice      string   `json:"voice"`
	Status     string   `json:"status"`
	Objective  string   `json:"objective"`
	Guardrails []string `json:"guardrails"`
}

type campaign struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId"`
	Name        string `json:"name"`
	BotID       string `json:"botId"`
	Status      string `json:"status"`
	Schedule    string `json:"schedule"`
	LeadCount   int    `json:"leadCount"`
	MaxAttempts int    `json:"maxAttempts"`
}

type call struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId"`
	LeadName    string `json:"leadName"`
	Phone       string `json:"phone"`
	Campaign    string `json:"campaign"`
	Status      string `json:"status"`
	Outcome     string `json:"outcome"`
	DurationSec int    `json:"durationSec"`
	StartedAt   string `json:"startedAt"`
	Summary     string `json:"summary"`
}

type store struct {
	mu         sync.RWMutex
	tenants    []tenant
	leads      []lead
	properties []property
	bots       []bot
	campaigns  []campaign
	calls      []call
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	port := env("PORT", "8080")
	app := newStore()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/overview", app.handleOverview)
	mux.HandleFunc("GET /api/admin/tenants", app.handleTenants)
	mux.HandleFunc("GET /api/leads", app.handleLeads)
	mux.HandleFunc("POST /api/leads", app.handleCreateLead)
	mux.HandleFunc("GET /api/properties", app.handleProperties)
	mux.HandleFunc("GET /api/bots", app.handleBots)
	mux.HandleFunc("GET /api/campaigns", app.handleCampaigns)
	mux.HandleFunc("POST /api/campaigns", app.handleCreateCampaign)
	mux.HandleFunc("GET /api/calls", app.handleCalls)
	mux.HandleFunc("POST /api/calls/test", app.handleTestCall)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           cors(logRequests(logger, mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting api", "port", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func newStore() *store {
	now := time.Now().UTC().Format(time.RFC3339)
	return &store{
		tenants: []tenant{
			{ID: "atrium", Name: "Atrium", Status: "active", Plan: "platform", CreatedAt: now},
			{ID: "demo-homes", Name: "Demo Homes", Status: "active", Plan: "growth", CreatedAt: now},
		},
		leads: []lead{
			{ID: "lead_001", TenantID: "atrium", Name: "Maria Lopez", Phone: "+1 555 0101", Email: "maria@example.com", Type: "renter", Status: "qualified", Source: "webform", Consent: "lead_form", LastActivity: now},
			{ID: "lead_002", TenantID: "atrium", Name: "Carlos Rivera", Phone: "+1 555 0102", Email: "carlos@example.com", Type: "owner", Status: "new", Source: "crm", Consent: "existing_lead", LastActivity: now},
			{ID: "lead_003", TenantID: "atrium", Name: "Ana Torres", Phone: "+1 555 0103", Email: "ana@example.com", Type: "renter", Status: "callback", Source: "portal", Consent: "lead_form", LastActivity: now},
		},
		properties: []property{
			{ID: "prop_001", TenantID: "atrium", Name: "Sunset Villas 2B", Address: "Miami, FL", Price: "$2,450/mo", Availability: "Available now", Requirements: []string{"Income 3x rent", "Background check", "Application fee"}, FAQs: []string{"Pets allowed with deposit", "Parking included"}},
			{ID: "prop_002", TenantID: "atrium", Name: "Downtown Studio", Address: "Orlando, FL", Price: "$1,650/mo", Availability: "June 1", Requirements: []string{"Income verification", "No evictions"}, FAQs: []string{"Utilities separate", "12 month lease"}},
		},
		bots: []bot{
			{ID: "bot_001", TenantID: "atrium", Name: "Leasing Assistant", Type: "renter_inbound", Language: "es-US", Voice: "warm", Status: "draft", Objective: "Qualify renters and explain application requirements", Guardrails: []string{"Disclose AI assistant", "Do not invent pricing", "Transfer sensitive questions"}},
			{ID: "bot_002", TenantID: "atrium", Name: "Owner Outreach", Type: "owner_outbound", Language: "en-US", Voice: "confident", Status: "draft", Objective: "Explain property management service and schedule human follow-up", Guardrails: []string{"Respect opt-out", "Use approved claims only"}},
		},
		campaigns: []campaign{
			{ID: "camp_001", TenantID: "atrium", Name: "Renter follow-up", BotID: "bot_001", Status: "scheduled", Schedule: "Weekdays 10:00-18:00", LeadCount: 32, MaxAttempts: 3},
			{ID: "camp_002", TenantID: "atrium", Name: "Owner warm leads", BotID: "bot_002", Status: "paused", Schedule: "Tue/Thu 11:00-16:00", LeadCount: 14, MaxAttempts: 2},
		},
		calls: []call{
			{ID: "call_001", TenantID: "atrium", LeadName: "Maria Lopez", Phone: "+1 555 0101", Campaign: "Renter follow-up", Status: "completed", Outcome: "qualified", DurationSec: 286, StartedAt: now, Summary: "Interested in Sunset Villas. Move-in next month. Needs pet policy confirmation."},
			{ID: "call_002", TenantID: "atrium", LeadName: "Ana Torres", Phone: "+1 555 0103", Campaign: "Renter follow-up", Status: "completed", Outcome: "callback", DurationSec: 91, StartedAt: now, Summary: "Asked for callback after 17:00 with spouse present."},
			{ID: "call_003", TenantID: "atrium", LeadName: "Carlos Rivera", Phone: "+1 555 0102", Campaign: "Owner warm leads", Status: "queued", Outcome: "pending", DurationSec: 0, StartedAt: "", Summary: ""},
		},
	}
}

func (s *store) handleOverview(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"callsToday":      len(s.calls),
		"qualifiedLeads":  countCallsByOutcome(s.calls, "qualified"),
		"callbacks":       countCallsByOutcome(s.calls, "callback"),
		"activeCampaigns": countCampaignsByStatus(s.campaigns, "scheduled"),
		"queuedCalls":     countCallsByStatus(s.calls, "queued"),
	})
}

func (s *store) handleTenants(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.tenants)
}

func (s *store) handleLeads(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.leads)
}

func (s *store) handleCreateLead(w http.ResponseWriter, r *http.Request) {
	var input lead
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Phone) == "" {
		writeError(w, http.StatusBadRequest, "name_and_phone_required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	input.ID = "lead_" + time.Now().UTC().Format("20060102150405")
	if input.TenantID == "" {
		input.TenantID = "atrium"
	}
	if input.Status == "" {
		input.Status = "new"
	}
	input.LastActivity = time.Now().UTC().Format(time.RFC3339)
	s.leads = append([]lead{input}, s.leads...)
	writeJSON(w, http.StatusCreated, input)
}

func (s *store) handleProperties(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.properties)
}

func (s *store) handleBots(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.bots)
}

func (s *store) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.campaigns)
}

func (s *store) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	var input campaign
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	input.ID = "camp_" + time.Now().UTC().Format("20060102150405")
	if input.TenantID == "" {
		input.TenantID = "atrium"
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	s.campaigns = append([]campaign{input}, s.campaigns...)
	writeJSON(w, http.StatusCreated, input)
}

func (s *store) handleCalls(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.calls)
}

func (s *store) handleTestCall(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"message": "Test call accepted. Asterisk originate will be wired in the next iteration.",
	})
}

func countCallsByOutcome(calls []call, outcome string) int {
	count := 0
	for _, c := range calls {
		if c.Outcome == outcome {
			count++
		}
	}
	return count
}

func countCallsByStatus(calls []call, status string) int {
	count := 0
	for _, c := range calls {
		if c.Status == status {
			count++
		}
	}
	return count
}

func countCampaignsByStatus(campaigns []campaign, status string) int {
	count := 0
	for _, c := range campaigns {
		if c.Status == status {
			count++
		}
	}
	return count
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
