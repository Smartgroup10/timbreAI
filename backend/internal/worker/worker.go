package worker

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"timbre/backend/internal/store"
)

// Worker scans queued calls and decides whether each one is eligible to dispatch right now.
// Eligibility checks:
//   1. Phone is not in the tenant's Do Not Call list.
//   2. Tenant's allowed_hours window covers the current moment (in the tenant's timezone).
//   3. Today's call count for the tenant has not exceeded daily_call_cap.
//
// Calls that fail (1) are marked as skipped/blocked. Calls that fail (2) or (3) are left in the
// queue to retry on a future tick.
type Worker struct {
	store    *store.Store
	logger   *slog.Logger
	interval time.Duration
}

func New(st *store.Store, logger *slog.Logger, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{store: st, logger: logger, interval: interval}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started", "interval", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	// Run once on boot so a freshly seeded DB gets evaluated immediately.
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	w.expandCampaigns(ctx)

	queued, err := w.store.QueuedCalls(ctx, 50)
	if err != nil {
		w.logger.Warn("worker list queued", "error", err)
		return
	}
	if len(queued) == 0 {
		return
	}
	// Cache tenant settings within a tick — most calls share the same tenant.
	settingsCache := map[string]store.TenantSettings{}
	countsCache := map[string]int{}

	for _, call := range queued {
		ts, ok := settingsCache[call.TenantID]
		if !ok {
			t, err := w.store.GetTenantSettings(ctx, call.TenantID)
			if err != nil {
				w.logger.Warn("worker tenant settings", "tenant", call.TenantID, "error", err)
				continue
			}
			settingsCache[call.TenantID] = t
			ts = t
		}

		// 1) DNC check.
		blocked, err := w.store.IsBlockedPhone(ctx, call.TenantID, call.Phone)
		if err == nil && blocked {
			if err := w.store.MarkCallSkipped(ctx, call.ID, "dnc_blocked", "Number in do_not_call list at dispatch time."); err == nil {
				w.logger.Info("worker blocked by dnc", "call", call.ID, "phone", call.Phone)
			}
			continue
		}

		// 2) Hours/days check.
		if !withinWindow(ts) {
			w.logger.Debug("worker out of window", "call", call.ID, "tenant", call.TenantID)
			continue
		}

		// 3) Daily cap.
		count, ok := countsCache[call.TenantID]
		if !ok {
			n, err := w.store.CountCallsToday(ctx, call.TenantID, ts.Timezone)
			if err != nil {
				w.logger.Warn("worker count today", "tenant", call.TenantID, "error", err)
				continue
			}
			countsCache[call.TenantID] = n
			count = n
		}
		if ts.DailyCallCap > 0 && count >= ts.DailyCallCap {
			w.logger.Debug("worker cap reached", "tenant", call.TenantID, "cap", ts.DailyCallCap)
			continue
		}

		// All checks passed. The actual ARI Originate happens inline in handleTestCall when a
		// user triggers a manual test; the worker just marks the call eligible. When the campaign
		// expander lands, this is where we'd call s.ari.Originate with the bot/DID context. For
		// now we leave the call queued and bump the counter so cap math stays correct.
		countsCache[call.TenantID] = count + 1
		w.logger.Info("worker eligible", "call", call.ID, "tenant", call.TenantID, "phone", redactPhone(call.Phone))
	}
}

// withinWindow checks whether the current moment in the tenant's timezone falls inside its
// allowed_hours_start..allowed_hours_end on an allowed weekday.
func withinWindow(ts store.TenantSettings) bool {
	loc, err := time.LoadLocation(ts.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	if len(ts.AllowedDays) > 0 {
		want := strings.ToLower(now.Weekday().String()[:3])
		ok := false
		for _, d := range ts.AllowedDays {
			if strings.ToLower(strings.TrimSpace(d)) == want {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}

	start := parseHM(ts.AllowedHoursStart, 0, 0)
	end := parseHM(ts.AllowedHoursEnd, 23, 59)
	cur := now.Hour()*60 + now.Minute()
	startMin := start.h*60 + start.m
	endMin := end.h*60 + end.m
	return cur >= startMin && cur < endMin
}

type hm struct{ h, m int }

func parseHM(s string, defH, defM int) hm {
	if len(s) < 4 {
		return hm{defH, defM}
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return hm{defH, defM}
	}
	return hm{t.Hour(), t.Minute()}
}

func redactPhone(p string) string {
	if len(p) < 4 {
		return "***"
	}
	return "***" + p[len(p)-4:]
}

// expandCampaigns picks campaign_leads that are due for a new attempt and creates queued call
// rows for them. Each created call is then evaluated by the regular dispatcher tick below for
// eligibility (DNC, hours, cap) — keeping all the gating logic in one place.
func (w *Worker) expandCampaigns(ctx context.Context) {
	due, err := w.store.NextDispatchableForCampaign(ctx, 50)
	if err != nil {
		w.logger.Warn("worker expand campaigns", "error", err)
		return
	}
	for _, d := range due {
		// DNC check before we even create the call so we don't poison the calls table.
		blocked, _ := w.store.IsBlockedPhone(ctx, d.TenantID, d.LeadPhone)
		if blocked {
			_ = w.store.SkipCampaignLead(ctx, d.CampaignLeadID, "dnc_blocked")
			w.logger.Info("expander dnc skip", "campaign", d.CampaignID, "lead", d.LeadID)
			continue
		}
		call, err := w.store.MarkCampaignLeadDispatched(ctx, d)
		if err != nil {
			w.logger.Warn("expander mark dispatched", "error", err, "campaign_lead", d.CampaignLeadID)
			continue
		}
		w.logger.Info("expander queued",
			"campaign", d.CampaignID, "call", call.ID, "phone", redactPhone(d.LeadPhone))
	}
}
