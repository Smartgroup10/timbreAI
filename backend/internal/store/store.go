package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store struct {
	pool       *pgxpool.Pool
	secretsKey []byte // master key para cifrar API keys de proveedores de voz (AES-256-GCM)
}

// New construye un Store. secretsKey debe ser de 32 bytes (AES-256) — viene de
// config.SecretsMasterKey. Si es nil/<32B los métodos que cifran fallarán.
func New(pool *pgxpool.Pool, secretsKey []byte) *Store {
	return &Store{pool: pool, secretsKey: secretsKey}
}

// --- Tenants ---

func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, status, plan, created_at FROM tenants ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tenant{}
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Status, &t.Plan, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// EnsureTenant es un upsert idempotente del tenant principal. Usado por el
// bootstrap en main.go: si BOOTSTRAP_TENANT_ID/NAME cambian en el .env tras
// el primer arranque, el nombre se sincroniza pero el resto de columnas se
// dejan intactas (por si el operador las editó desde la UI admin).
func (s *Store) EnsureTenant(ctx context.Context, id, name string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tenants (id, name, status, plan)
		VALUES ($1, $2, 'active', 'platform')
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
		id, name)
	return err
}

func (s *Store) CreateTenant(ctx context.Context, id, name, plan, status string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tenants (id, name, plan, status) VALUES ($1, $2, $3, $4)`,
		id, name, plan, status)
	if err != nil {
		// unique_violation
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrConflict
		}
		return err
	}
	// Seed default tenant_settings so the new tenant lands ready to use.
	_, _ = s.pool.Exec(ctx, `INSERT INTO tenant_settings (tenant_id) VALUES ($1) ON CONFLICT DO NOTHING`, id)
	return nil
}

func (s *Store) UpdateTenant(ctx context.Context, id string, name, plan, status *string) error {
	set := []string{}
	args := []any{id}
	if name != nil {
		args = append(args, *name)
		set = append(set, "name = $"+itoaCheap(len(args)))
	}
	if plan != nil {
		args = append(args, *plan)
		set = append(set, "plan = $"+itoaCheap(len(args)))
	}
	if status != nil {
		args = append(args, *status)
		set = append(set, "status = $"+itoaCheap(len(args)))
	}
	if len(set) == 0 {
		return nil
	}
	q := "UPDATE tenants SET " + strings.Join(set, ", ") + " WHERE id = $1"
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// itoaCheap is a small helper for query builders to avoid pulling in fmt for each placeholder.
func itoaCheap(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoaSlow(n)
}

func itoaSlow(n int) string {
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func (s *Store) GetTenant(ctx context.Context, id string) (Tenant, error) {
	var t Tenant
	err := s.pool.QueryRow(ctx, `SELECT id, name, status, plan, created_at FROM tenants WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Status, &t.Plan, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// --- Users ---

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, email, name, role, password_hash, last_login_at, created_at
		FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.LastLoginAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, u User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, name, role, password_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (email) DO NOTHING`,
		u.ID, u.TenantID, u.Email, u.Name, u.Role, u.PasswordHash)
	return err
}

func (s *Store) TouchUserLogin(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

// --- Leads ---

func (s *Store) ListLeads(ctx context.Context, tenantID string) ([]Lead, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, phone, email, type, status, source, consent, last_activity
		FROM leads WHERE tenant_id = $1 ORDER BY last_activity DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lead{}
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.TenantID, &l.Name, &l.Phone, &l.Email, &l.Type, &l.Status, &l.Source, &l.Consent, &l.LastActivity); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) CreateLead(ctx context.Context, l Lead) (Lead, error) {
	if l.ID == "" {
		l.ID = newID("lead")
	}
	if l.Status == "" {
		l.Status = "new"
	}
	l.LastActivity = time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO leads (id, tenant_id, name, phone, email, type, status, source, consent, last_activity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		l.ID, l.TenantID, l.Name, l.Phone, l.Email, l.Type, l.Status, l.Source, l.Consent, l.LastActivity)
	return l, err
}

// --- Properties ---

func (s *Store) ListProperties(ctx context.Context, tenantID string) ([]Property, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, address, price, availability, requirements, faqs
		FROM properties WHERE tenant_id = $1 ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Property{}
	for rows.Next() {
		var p Property
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Address, &p.Price, &p.Availability, &p.Requirements, &p.FAQs); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- Bots ---

func (s *Store) ListBots(ctx context.Context, tenantID string) ([]Bot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.tenant_id, b.name, b.type, b.language, b.voice, b.status, b.objective, b.guardrails,
		       b.voice_provider, b.did_id, COALESCE(d.e164, ''), COALESCE(d.trunk_id, '')
		FROM bots b
		LEFT JOIN dids d ON d.id = b.did_id
		WHERE b.tenant_id = $1
		ORDER BY b.name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bot{}
	for rows.Next() {
		var b Bot
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.Type, &b.Language, &b.Voice, &b.Status, &b.Objective, &b.Guardrails,
			&b.VoiceProvider, &b.DIDID, &b.DIDE164, &b.TrunkID); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBotByID busca un bot sin filtrar por tenant. Para uso de platform_admin
// (test calls cross-tenant, etc.); los handlers que reciben tenant del JWT
// deben seguir usando GetBot(ctx, tenantID, id).
func (s *Store) GetBotByID(ctx context.Context, id string) (Bot, error) {
	var b Bot
	err := s.pool.QueryRow(ctx, `
		SELECT b.id, b.tenant_id, b.name, b.type, b.language, b.voice, b.status, b.objective, b.guardrails,
		       b.voice_provider, b.did_id, COALESCE(d.e164, ''), COALESCE(d.trunk_id, '')
		FROM bots b
		LEFT JOIN dids d ON d.id = b.did_id
		WHERE b.id = $1`, id).
		Scan(&b.ID, &b.TenantID, &b.Name, &b.Type, &b.Language, &b.Voice, &b.Status, &b.Objective, &b.Guardrails,
			&b.VoiceProvider, &b.DIDID, &b.DIDE164, &b.TrunkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

func (s *Store) GetBot(ctx context.Context, tenantID, id string) (Bot, error) {
	var b Bot
	err := s.pool.QueryRow(ctx, `
		SELECT b.id, b.tenant_id, b.name, b.type, b.language, b.voice, b.status, b.objective, b.guardrails,
		       b.voice_provider, b.did_id, COALESCE(d.e164, ''), COALESCE(d.trunk_id, '')
		FROM bots b
		LEFT JOIN dids d ON d.id = b.did_id
		WHERE b.tenant_id = $1 AND b.id = $2`, tenantID, id).
		Scan(&b.ID, &b.TenantID, &b.Name, &b.Type, &b.Language, &b.Voice, &b.Status, &b.Objective, &b.Guardrails,
			&b.VoiceProvider, &b.DIDID, &b.DIDE164, &b.TrunkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// AssignBotDID sets the DID a bot will use as caller-id when originating outbound calls.
// A nil didID clears the assignment. The DID must belong to the same tenant as the bot.
func (s *Store) AssignBotDID(ctx context.Context, tenantID, botID string, didID *string) error {
	if didID != nil && *didID != "" {
		var ownerTenant *string
		err := s.pool.QueryRow(ctx, `SELECT tenant_id FROM dids WHERE id = $1`, *didID).Scan(&ownerTenant)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if ownerTenant == nil || *ownerTenant != tenantID {
			return errors.New("did_not_assigned_to_tenant")
		}
	}
	tag, err := s.pool.Exec(ctx, `UPDATE bots SET did_id = $3 WHERE tenant_id = $1 AND id = $2`, tenantID, botID, didID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Campaigns ---

func (s *Store) ListCampaigns(ctx context.Context, tenantID string) ([]Campaign, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, COALESCE(bot_id, ''), name, status, schedule, lead_count,
		       max_attempts, retry_cooldown_minutes, start_at, end_at, max_concurrent
		FROM campaigns WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Campaign{}
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.TenantID, &c.BotID, &c.Name, &c.Status, &c.Schedule, &c.LeadCount,
			&c.MaxAttempts, &c.RetryCooldownMinutes, &c.StartAt, &c.EndAt, &c.MaxConcurrent); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCampaign(ctx context.Context, c Campaign) (Campaign, error) {
	if c.ID == "" {
		c.ID = newID("camp")
	}
	if c.Status == "" {
		c.Status = "draft"
	}
	if c.MaxAttempts == 0 {
		c.MaxAttempts = 1
	}
	if c.MaxConcurrent == 0 {
		c.MaxConcurrent = 3
	}
	var botID any
	if c.BotID != "" {
		botID = c.BotID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO campaigns (id, tenant_id, bot_id, name, status, schedule, lead_count,
		                     max_attempts, start_at, end_at, max_concurrent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		c.ID, c.TenantID, botID, c.Name, c.Status, c.Schedule, c.LeadCount,
		c.MaxAttempts, c.StartAt, c.EndAt, c.MaxConcurrent)
	return c, err
}

// --- Calls ---

func (s *Store) ListCalls(ctx context.Context, tenantID string, limit int) ([]Call, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		       duration_sec, channel_id, voice_session_id, started_at, ended_at, summary, recording_url, provider
		FROM calls
		WHERE tenant_id = $1
		ORDER BY COALESCE(started_at, created_at) DESC
		LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.ID, &c.TenantID, &c.LeadID, &c.CampaignID, &c.LeadName, &c.Campaign, &c.Phone, &c.Status, &c.Outcome,
			&c.DurationSec, &c.ChannelID, &c.VoiceSessionID, &c.StartedAt, &c.EndedAt, &c.Summary, &c.RecordingURL, &c.Provider); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCall(ctx context.Context, tenantID, id string) (Call, error) {
	var c Call
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		       duration_sec, channel_id, voice_session_id, started_at, ended_at, summary, recording_url, provider
		FROM calls WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&c.ID, &c.TenantID, &c.LeadID, &c.CampaignID, &c.LeadName, &c.Campaign, &c.Phone, &c.Status, &c.Outcome,
			&c.DurationSec, &c.ChannelID, &c.VoiceSessionID, &c.StartedAt, &c.EndedAt, &c.Summary, &c.RecordingURL, &c.Provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) CreateCall(ctx context.Context, c Call) (Call, error) {
	if c.ID == "" {
		c.ID = newID("call")
	}
	if c.Status == "" {
		c.Status = "queued"
	}
	if c.Outcome == "" {
		c.Outcome = "pending"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO calls (id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		                   duration_sec, channel_id, started_at, summary, provider)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		c.ID, c.TenantID, c.LeadID, c.CampaignID, c.LeadName, c.Campaign, c.Phone, c.Status, c.Outcome,
		c.DurationSec, c.ChannelID, c.StartedAt, c.Summary, c.Provider)
	return c, err
}

func (s *Store) UpdateCallChannel(ctx context.Context, tenantID, id, channelID, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE calls SET channel_id = $3, status = $4, started_at = COALESCE(started_at, now())
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, channelID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetCallRecording(ctx context.Context, tenantID, id, url string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE calls SET recording_url = $3 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, url)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetCallVoiceSession(ctx context.Context, tenantID, id, sessionID string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE calls SET voice_session_id = $3 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, sessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FindCallByChannel finds the call that owns a given Asterisk channel id. Used by the Stasis
// handler to correlate the StasisStart event back to a tenant/bot context.
func (s *Store) FindCallByChannel(ctx context.Context, channelID string) (Call, error) {
	var c Call
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		       duration_sec, channel_id, voice_session_id, started_at, ended_at, summary, recording_url, provider
		FROM calls WHERE channel_id = $1`, channelID).
		Scan(&c.ID, &c.TenantID, &c.LeadID, &c.CampaignID, &c.LeadName, &c.Campaign, &c.Phone, &c.Status, &c.Outcome,
			&c.DurationSec, &c.ChannelID, &c.VoiceSessionID, &c.StartedAt, &c.EndedAt, &c.Summary, &c.RecordingURL, &c.Provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// MarkCallAnswered transitions a dialing call to "answered" once the External Media bridge is in
// place. We also stash the ExternalMedia channel id in summary metadata for debugging.
func (s *Store) MarkCallAnswered(ctx context.Context, tenantID, id, externalMediaID, bridgeID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE calls SET status = 'answered',
		                 started_at = COALESCE(started_at, now()),
		                 summary = CASE WHEN summary = '' OR summary IS NULL
		                                THEN $3 ELSE summary END
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, "Bridged via External Media (em="+externalMediaID+", bridge="+bridgeID+")")
	return err
}

func (s *Store) FinishCall(ctx context.Context, channelID, status, outcome, summary string, durationSec int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE calls
		SET status = $2, outcome = COALESCE(NULLIF($3, ''), outcome),
		    summary = COALESCE(NULLIF($4, ''), summary), duration_sec = $5, ended_at = now()
		WHERE channel_id = $1`, channelID, status, outcome, summary, durationSec)
	return err
}

// QueuedCalls returns calls that are waiting to be dispatched, oldest first. Bounded by `limit`.
func (s *Store) QueuedCalls(ctx context.Context, limit int) ([]Call, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		       duration_sec, channel_id, voice_session_id, started_at, ended_at, summary, recording_url, provider
		FROM calls WHERE status = 'queued'
		ORDER BY created_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.ID, &c.TenantID, &c.LeadID, &c.CampaignID, &c.LeadName, &c.Campaign, &c.Phone,
			&c.Status, &c.Outcome, &c.DurationSec, &c.ChannelID, &c.VoiceSessionID, &c.StartedAt, &c.EndedAt,
			&c.Summary, &c.RecordingURL, &c.Provider); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountCallsToday returns the number of calls dispatched (started_at != null) for a tenant today
// in the tenant's local timezone. Used by the worker to enforce daily_call_cap.
func (s *Store) CountCallsToday(ctx context.Context, tenantID, timezone string) (int, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM calls
		WHERE tenant_id = $1 AND started_at IS NOT NULL
		  AND (started_at AT TIME ZONE $2)::date = (now() AT TIME ZONE $2)::date`, tenantID, timezone).Scan(&n)
	return n, err
}

// MarkCallSkipped transitions a queued call to a final state with an explanation. Used by the
// worker when a call fails eligibility (DNC, outside hours, cap reached).
func (s *Store) MarkCallSkipped(ctx context.Context, id, outcome, reason string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE calls SET status = 'skipped', outcome = $2, summary = $3, ended_at = now()
		WHERE id = $1 AND status = 'queued'`, id, outcome, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) IsBlockedPhone(ctx context.Context, tenantID, phone string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM do_not_call WHERE tenant_id = $1 AND phone = $2`, tenantID, phone).Scan(&n)
	return n > 0, err
}

// --- Overview ---

func (s *Store) Overview(ctx context.Context, tenantID string) (Overview, error) {
	var o Overview
	err := s.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM calls WHERE tenant_id = $1 AND COALESCE(started_at, created_at) >= date_trunc('day', now())),
		  (SELECT count(*) FROM calls WHERE tenant_id = $1 AND outcome = 'qualified'),
		  (SELECT count(*) FROM calls WHERE tenant_id = $1 AND outcome = 'callback'),
		  (SELECT count(*) FROM campaigns WHERE tenant_id = $1 AND status = 'active'),
		  (SELECT count(*) FROM calls WHERE tenant_id = $1 AND status = 'queued')`, tenantID).
		Scan(&o.CallsToday, &o.QualifiedLeads, &o.Callbacks, &o.ActiveCampaigns, &o.QueuedCalls)
	if err != nil {
		return o, fmt.Errorf("overview: %w", err)
	}
	return o, nil
}
