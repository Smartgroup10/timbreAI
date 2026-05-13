// Package worker corre el dispatcher de campañas: cada `interval` (30s por
// defecto) mira qué campaign_leads están listos para ser llamados, valida la
// elegibilidad (DNC, horario, daily cap) y origina la llamada vía ARI.
//
// La concurrencia se controla con un semáforo per-campaña basado en
// campaigns.max_concurrent. Cuando una llamada termina (ChannelDestroyed),
// el ARI handler llama a worker.ReleaseSlot(callID) para liberar el cupo.
package worker

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"timbre/backend/internal/app"
	"timbre/backend/internal/store"
)

type Worker struct {
	deps     app.DialDeps
	logger   *slog.Logger
	interval time.Duration

	mu       sync.Mutex
	sems     map[string]chan struct{} // campaignID → semáforo (buffered chan)
	inFlight map[string]string        // callID → campaignID (para liberar al colgar)
}

func New(deps app.DialDeps, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{
		deps:     deps,
		logger:   deps.Logger,
		interval: interval,
		sems:     map[string]chan struct{}{},
		inFlight: map[string]string{},
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started", "interval", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
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

// ReleaseSlot libera el cupo del semáforo asociado a callID. Idempotente:
// llamadas repetidas no hacen daño.
func (w *Worker) ReleaseSlot(callID string) {
	w.mu.Lock()
	campaignID, ok := w.inFlight[callID]
	if ok {
		delete(w.inFlight, callID)
	}
	sem := w.sems[campaignID]
	w.mu.Unlock()
	if !ok || sem == nil {
		return
	}
	select {
	case <-sem:
	default:
	}
}

// tryAcquire intenta tomar un slot del semáforo de campaignID. Si el semáforo
// no existe lo crea con la capacidad pedida. Retorna false si está lleno.
func (w *Worker) tryAcquire(campaignID string, maxConcurrent int) bool {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	w.mu.Lock()
	sem, ok := w.sems[campaignID]
	// Si la campaña cambió max_concurrent en BD, recreamos el sem con la nueva
	// capacidad (mientras esté vacío). Si está ocupado, esperamos al próximo
	// tick para recrearlo.
	if !ok || (cap(sem) != maxConcurrent && len(sem) == 0) {
		sem = make(chan struct{}, maxConcurrent)
		w.sems[campaignID] = sem
	}
	w.mu.Unlock()
	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (w *Worker) trackInFlight(callID, campaignID string) {
	w.mu.Lock()
	w.inFlight[callID] = campaignID
	w.mu.Unlock()
}

func (w *Worker) tick(ctx context.Context) {
	w.expandAndDial(ctx)
}

// expandAndDial:
//  1. Pide a la BD los campaign_leads listos para llamar (status=active,
//     start_at/end_at OK, attempts < max, cooldown OK).
//  2. Para cada uno: valida DNC + horario + daily cap + semáforo. Si todo OK,
//     crea la fila en calls (MarkCampaignLeadDispatched) y dispara DialCall
//     en goroutine.
func (w *Worker) expandAndDial(ctx context.Context) {
	due, err := w.deps.Store.NextDispatchableForCampaign(ctx, 50)
	if err != nil {
		w.logger.Warn("worker dispatch list", "error", err)
		return
	}
	if len(due) == 0 {
		return
	}
	settingsCache := map[string]store.TenantSettings{}
	countsCache := map[string]int{}

	for _, d := range due {
		// Tenant settings: horario y daily cap.
		ts, ok := settingsCache[d.TenantID]
		if !ok {
			t, err := w.deps.Store.GetTenantSettings(ctx, d.TenantID)
			if err != nil {
				w.logger.Warn("worker tenant settings", "tenant", d.TenantID, "error", err)
				continue
			}
			settingsCache[d.TenantID] = t
			ts = t
		}
		if !withinWindow(ts) {
			continue
		}

		// DNC.
		blocked, _ := w.deps.Store.IsBlockedPhone(ctx, d.TenantID, d.LeadPhone)
		if blocked {
			_ = w.deps.Store.SkipCampaignLead(ctx, d.CampaignLeadID, "dnc_blocked")
			w.logger.Info("dispatcher dnc skip", "campaign", d.CampaignID, "lead", d.LeadID)
			continue
		}

		// Daily cap.
		count, ok := countsCache[d.TenantID]
		if !ok {
			n, err := w.deps.Store.CountCallsToday(ctx, d.TenantID, ts.Timezone)
			if err != nil {
				w.logger.Warn("worker count today", "error", err)
				continue
			}
			countsCache[d.TenantID] = n
			count = n
		}
		if ts.DailyCallCap > 0 && count >= ts.DailyCallCap {
			w.logger.Debug("worker cap reached", "tenant", d.TenantID, "cap", ts.DailyCallCap)
			continue
		}

		// Concurrencia per-campaña.
		if !w.tryAcquire(d.CampaignID, d.MaxConcurrent) {
			continue // probaremos en el próximo tick
		}
		countsCache[d.TenantID] = count + 1

		// Crear la fila en calls + bumpear campaign_lead.
		call, err := w.deps.Store.MarkCampaignLeadDispatched(ctx, d)
		if err != nil {
			w.releaseImmediate(d.CampaignID)
			w.logger.Warn("worker mark dispatched", "error", err, "campaign_lead", d.CampaignLeadID)
			continue
		}
		w.trackInFlight(call.ID, d.CampaignID)

		// Originate en background — no bloqueamos el tick.
		go w.dialInBackground(ctx, call, d.BotID)
	}
}

func (w *Worker) dialInBackground(ctx context.Context, call store.Call, botID string) {
	res, err := app.DialCall(ctx, w.deps, call, botID)
	if err != nil {
		w.logger.Error("worker dial failed", "call", call.ID, "error", err)
		_ = w.deps.Store.MarkCallSkipped(ctx, call.ID, "originate_failed", err.Error())
		w.ReleaseSlot(call.ID) // libera el slot — no esperamos ChannelDestroyed
		return
	}
	w.logger.Info("worker call originated", "call", call.ID, "channel", res.ChannelID, "phone", redactPhone(call.Phone))
}

// releaseImmediate libera un slot tomado por tryAcquire en el mismo tick si
// algo falló antes de poder rastrear el callID.
func (w *Worker) releaseImmediate(campaignID string) {
	w.mu.Lock()
	sem := w.sems[campaignID]
	w.mu.Unlock()
	if sem == nil {
		return
	}
	select {
	case <-sem:
	default:
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
