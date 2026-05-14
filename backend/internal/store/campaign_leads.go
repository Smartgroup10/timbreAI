package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AddLeadsToCampaign inserts lead-campaign rows in bulk. Existing rows (same campaign+lead pair)
// are ignored. Returns the number of new rows created and the up-to-date campaign lead_count.
func (s *Store) AddLeadsToCampaign(ctx context.Context, tenantID, campaignID string, leadIDs []string) (int, int, error) {
	if len(leadIDs) == 0 {
		return 0, 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	// Verify campaign belongs to tenant.
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM campaigns WHERE tenant_id = $1 AND id = $2`, tenantID, campaignID).Scan(&n); err != nil {
		return 0, 0, err
	}
	if n == 0 {
		return 0, 0, ErrNotFound
	}

	// Build a multi-row insert in one round-trip.
	args := make([]any, 0, len(leadIDs)*4)
	values := make([]string, 0, len(leadIDs))
	for i, leadID := range leadIDs {
		off := i * 4
		args = append(args, newID("cl"), tenantID, campaignID, leadID)
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d)", off+1, off+2, off+3, off+4))
	}
	q := `INSERT INTO campaign_leads (id, tenant_id, campaign_id, lead_id) VALUES ` +
		strings.Join(values, ", ") +
		` ON CONFLICT (campaign_id, lead_id) DO NOTHING`
	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return 0, 0, err
	}
	created := int(tag.RowsAffected())

	// Refresh denormalized lead_count on the campaign row.
	var total int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM campaign_leads WHERE campaign_id = $1`, campaignID).Scan(&total); err != nil {
		return created, 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE campaigns SET lead_count = $2 WHERE id = $1`, campaignID, total); err != nil {
		return created, total, err
	}
	return created, total, tx.Commit(ctx)
}

func (s *Store) RemoveLeadFromCampaign(ctx context.Context, tenantID, campaignID, leadID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM campaign_leads
		WHERE tenant_id = $1 AND campaign_id = $2 AND lead_id = $3`, tenantID, campaignID, leadID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	// Refresh lead_count (best effort).
	_, _ = s.pool.Exec(ctx, `
		UPDATE campaigns SET lead_count = (SELECT count(*) FROM campaign_leads WHERE campaign_id = $1)
		WHERE id = $1`, campaignID)
	return nil
}

func (s *Store) ListCampaignLeads(ctx context.Context, tenantID, campaignID string) ([]CampaignLead, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT cl.id, cl.tenant_id, cl.campaign_id, cl.lead_id, l.name, l.phone,
		       cl.status, cl.attempts, cl.last_attempt_at, cl.outcome
		FROM campaign_leads cl JOIN leads l ON l.id = cl.lead_id
		WHERE cl.tenant_id = $1 AND cl.campaign_id = $2
		ORDER BY cl.created_at`, tenantID, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CampaignLead{}
	for rows.Next() {
		var cl CampaignLead
		var last *time.Time
		if err := rows.Scan(&cl.ID, &cl.TenantID, &cl.CampaignID, &cl.LeadID, &cl.LeadName, &cl.LeadPhone,
			&cl.Status, &cl.Attempts, &last, &cl.Outcome); err != nil {
			return nil, err
		}
		if last != nil {
			s := last.UTC().Format(time.RFC3339)
			cl.LastAttemptAt = &s
		}
		out = append(out, cl)
	}
	return out, rows.Err()
}

// --- Campaign expander queries (called by the worker every tick) ---

// NextDispatchableForCampaign returns up to `limit` campaign_leads ready for a new attempt:
//   - campaign is scheduled
//   - cl.status in (pending, failed) and attempts < max_attempts
//   - cooldown has elapsed since last_attempt_at
//
// Rows are also locked with FOR UPDATE SKIP LOCKED so two worker replicas can run safely.
type DispatchableCampaignLead struct {
	CampaignLeadID string
	CampaignID     string
	TenantID       string
	LeadID         string
	LeadName       string
	LeadPhone      string
	CampaignName   string
	BotID          string
	BotProvider    string // snapshot voice_provider del bot, para coste
	MaxConcurrent  int
}

// NextDispatchableForCampaign devuelve hasta `limit` campaign_leads listos para
// ser marcados como llamada. Filtros:
//   - campaign.status = 'active' (los campaigns 'draft'/'paused'/'completed' se ignoran)
//   - start_at NULL o ya pasado
//   - end_at NULL o aún no llegado
//   - cl.status pending o failed (no calling/done/blocked)
//   - attempts < max_attempts
//   - cooldown elapsed
//
// FOR UPDATE SKIP LOCKED permite que dos réplicas del backend corran a la vez.
func (s *Store) NextDispatchableForCampaign(ctx context.Context, limit int) ([]DispatchableCampaignLead, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT cl.id, cl.campaign_id, cl.tenant_id, cl.lead_id, l.name, l.phone, c.name,
		       COALESCE(c.bot_id, ''), COALESCE(b.voice_provider, ''), c.max_concurrent
		FROM campaign_leads cl
		JOIN campaigns c ON c.id = cl.campaign_id
		JOIN leads l ON l.id = cl.lead_id
		LEFT JOIN bots b ON b.id = c.bot_id
		WHERE c.status = 'active'
		  AND (c.start_at IS NULL OR c.start_at <= now())
		  AND (c.end_at   IS NULL OR c.end_at   >= now())
		  AND cl.status IN ('pending', 'failed')
		  AND cl.attempts < c.max_attempts
		  AND (cl.last_attempt_at IS NULL
		       OR cl.last_attempt_at + (c.retry_cooldown_minutes::text || ' minutes')::interval <= now())
		ORDER BY cl.last_attempt_at NULLS FIRST, cl.created_at
		LIMIT $1
		FOR UPDATE OF cl SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DispatchableCampaignLead{}
	for rows.Next() {
		var d DispatchableCampaignLead
		if err := rows.Scan(&d.CampaignLeadID, &d.CampaignID, &d.TenantID, &d.LeadID,
			&d.LeadName, &d.LeadPhone, &d.CampaignName, &d.BotID, &d.BotProvider, &d.MaxConcurrent); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MarkCampaignLeadDispatched bumps attempts and last_attempt_at on a campaign_lead and creates
// the corresponding call row in a single transaction.
func (s *Store) MarkCampaignLeadDispatched(ctx context.Context, cl DispatchableCampaignLead) (Call, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Call{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE campaign_leads
		SET status = 'calling', attempts = attempts + 1, last_attempt_at = now()
		WHERE id = $1`, cl.CampaignLeadID); err != nil {
		return Call{}, err
	}

	provider := cl.BotProvider
	if provider == "" {
		provider = "echo" // sin bot asignado a la campaña, sandbox
	}
	call := Call{
		ID:         newID("call"),
		TenantID:   cl.TenantID,
		LeadID:     &cl.LeadID,
		CampaignID: &cl.CampaignID,
		LeadName:   cl.LeadName,
		Campaign:   cl.CampaignName,
		Phone:      cl.LeadPhone,
		Status:     "queued",
		Outcome:    "pending",
		Summary:    "Dispatched by campaign expander.",
		Provider:   provider,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO calls (id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome, summary, provider)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		call.ID, call.TenantID, call.LeadID, call.CampaignID, call.LeadName, call.Campaign, call.Phone, call.Status, call.Outcome, call.Summary, call.Provider,
	); err != nil {
		return Call{}, err
	}
	return call, tx.Commit(ctx)
}

// SkipCampaignLead marks a lead as blocked (typically DNC at dispatch time) so the worker won't
// pick it up again.
func (s *Store) SkipCampaignLead(ctx context.Context, campaignLeadID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE campaign_leads SET status = 'blocked', outcome = $2, last_attempt_at = now()
		WHERE id = $1`, campaignLeadID, reason)
	return err
}

var _ = errors.New
var _ pgx.Rows = nil
