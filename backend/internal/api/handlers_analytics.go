package api

import "net/http"

// analyticsResponse extiende store.Analytics con los costes calculados a
// partir de los segundos por provider y la tabla de tarifas. Lo hacemos
// en el handler (no en el store) para mantener el store libre de
// dependencias de pricing.
type analyticsResponse struct {
	Generated       any                       `json:"generatedAt"`
	Timezone        string                    `json:"timezone"`
	Last7Days       any                       `json:"last7Days"`
	Outcomes        any                       `json:"outcomes"`
	Statuses        any                       `json:"statuses"`
	TopBots         any                       `json:"topBots"`
	TopCampaigns    any                       `json:"topCampaigns"`
	TotalsLast7     int                       `json:"totalsLast7"`
	TotalsPrev7     int                       `json:"totalsPrev7"`
	ProviderSeconds any                       `json:"providerSeconds"`
	CostByProvider  []costByProviderBucket    `json:"costByProvider"`
	TotalCostCents  int                       `json:"totalCostCents"`
}

type costByProviderBucket struct {
	Provider    string `json:"provider"`
	Seconds     int    `json:"seconds"`
	CentsPerMin int    `json:"centsPerMin"`
	CostCents   int    `json:"costCents"`
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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

	// Calcula coste por provider y total. Útil para mostrar "Has gastado
	// X $ este mes en voz IA" en el dashboard.
	costs := make([]costByProviderBucket, 0, len(report.ProviderSeconds))
	total := 0
	for _, b := range report.ProviderSeconds {
		c := s.pricing.Cost(b.Provider, b.Seconds)
		costs = append(costs, costByProviderBucket{
			Provider:    b.Provider,
			Seconds:     b.Seconds,
			CentsPerMin: s.pricing.CentsPerMin(b.Provider),
			CostCents:   c,
		})
		total += c
	}

	writeJSON(w, http.StatusOK, analyticsResponse{
		Generated:       report.Generated,
		Timezone:        report.Timezone,
		Last7Days:       report.Last7Days,
		Outcomes:        report.Outcomes,
		Statuses:        report.Statuses,
		TopBots:         report.TopBots,
		TopCampaigns:    report.TopCampaigns,
		TotalsLast7:     report.TotalsLast7,
		TotalsPrev7:     report.TotalsPrev7,
		ProviderSeconds: report.ProviderSeconds,
		CostByProvider:  costs,
		TotalCostCents:  total,
	})
}

// handlePricing expone la tabla de tarifas actual para que el frontend
// pueda mostrar "Tarifa: 30¢/min" en el editor de bots, además de
// permitir tooltips informativos sobre el coste estimado.
func (s *Server) handlePricing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"centsPerMin": s.pricing.All(),
	})
}
