package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Lead mutations ---

type LeadPatch struct {
	Name    *string
	Phone   *string
	Email   *string
	Type    *string
	Status  *string
	Source  *string
	Consent *string
}

func (s *Store) UpdateLead(ctx context.Context, tenantID, id string, p LeadPatch) (Lead, error) {
	set := []string{"last_activity = now()"}
	args := []any{tenantID, id}
	add := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, *val)
		set = append(set, col+" = $"+itoa(len(args)))
	}
	add("name", p.Name)
	add("phone", p.Phone)
	add("email", p.Email)
	add("type", p.Type)
	add("status", p.Status)
	add("source", p.Source)
	add("consent", p.Consent)
	if len(set) == 1 {
		return s.getLead(ctx, tenantID, id)
	}
	q := "UPDATE leads SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1 AND id = $2"
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return Lead{}, err
	}
	if tag.RowsAffected() == 0 {
		return Lead{}, ErrNotFound
	}
	return s.getLead(ctx, tenantID, id)
}

func (s *Store) DeleteLead(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM leads WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetLead is the public flavour of getLead.
func (s *Store) GetLead(ctx context.Context, tenantID, id string) (Lead, error) {
	return s.getLead(ctx, tenantID, id)
}

// ListCallsForLead returns the calls associated with a lead (via calls.lead_id), newest first.
func (s *Store) ListCallsForLead(ctx context.Context, tenantID, leadID string) ([]Call, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		       duration_sec, channel_id, voice_session_id, started_at, ended_at, summary, recording_url
		FROM calls WHERE tenant_id = $1 AND lead_id = $2
		ORDER BY COALESCE(started_at, created_at) DESC
		LIMIT 100`, tenantID, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var c Call
		if err := rows.Scan(&c.ID, &c.TenantID, &c.LeadID, &c.CampaignID, &c.LeadName, &c.Campaign, &c.Phone,
			&c.Status, &c.Outcome, &c.DurationSec, &c.ChannelID, &c.VoiceSessionID, &c.StartedAt, &c.EndedAt,
			&c.Summary, &c.RecordingURL); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) getLead(ctx context.Context, tenantID, id string) (Lead, error) {
	var l Lead
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, phone, email, type, status, source, consent, last_activity
		FROM leads WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&l.ID, &l.TenantID, &l.Name, &l.Phone, &l.Email, &l.Type, &l.Status, &l.Source, &l.Consent, &l.LastActivity)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrNotFound
	}
	return l, err
}

// --- Bot mutations ---

type BotPatch struct {
	Name          *string
	Type          *string
	Language      *string
	Voice         *string
	Status        *string
	Objective     *string
	Guardrails    *[]string
	VoiceProvider *string
}

func (s *Store) CreateBot(ctx context.Context, b Bot) (Bot, error) {
	if b.ID == "" {
		b.ID = newID("bot")
	}
	if b.Status == "" {
		b.Status = "draft"
	}
	if b.Language == "" {
		b.Language = "es-ES"
	}
	if b.Voice == "" {
		b.Voice = "warm"
	}
	if b.Type == "" {
		b.Type = "renter_inbound"
	}
	if b.VoiceProvider == "" {
		b.VoiceProvider = "echo"
	}
	guardrails := b.Guardrails
	if guardrails == nil {
		guardrails = []string{}
	}
	guardrailsJSON, _ := json.Marshal(guardrails)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bots (id, tenant_id, name, type, language, voice, status, objective, guardrails, voice_provider)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)`,
		b.ID, b.TenantID, b.Name, b.Type, b.Language, b.Voice, b.Status, b.Objective, string(guardrailsJSON), b.VoiceProvider)
	if err != nil {
		return b, err
	}
	return s.GetBot(ctx, b.TenantID, b.ID)
}

func (s *Store) UpdateBot(ctx context.Context, tenantID, id string, p BotPatch) (Bot, error) {
	set := []string{}
	args := []any{tenantID, id}
	add := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, *val)
		set = append(set, col+" = $"+itoa(len(args)))
	}
	add("name", p.Name)
	add("type", p.Type)
	add("language", p.Language)
	add("voice", p.Voice)
	add("status", p.Status)
	add("objective", p.Objective)
	add("voice_provider", p.VoiceProvider)
	if p.Guardrails != nil {
		b, _ := json.Marshal(*p.Guardrails)
		args = append(args, string(b))
		set = append(set, "guardrails = $"+itoa(len(args))+"::jsonb")
	}
	if len(set) == 0 {
		return s.GetBot(ctx, tenantID, id)
	}
	q := "UPDATE bots SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1 AND id = $2"
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return Bot{}, err
	}
	if tag.RowsAffected() == 0 {
		return Bot{}, ErrNotFound
	}
	return s.GetBot(ctx, tenantID, id)
}

func (s *Store) DeleteBot(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM bots WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Campaign mutations ---

type CampaignPatch struct {
	Name        *string
	BotID       *string
	Status      *string
	Schedule    *string
	MaxAttempts *int
}

func (s *Store) UpdateCampaign(ctx context.Context, tenantID, id string, p CampaignPatch) (Campaign, error) {
	set := []string{}
	args := []any{tenantID, id}
	addStr := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, *val)
		set = append(set, col+" = $"+itoa(len(args)))
	}
	addStr("name", p.Name)
	if p.BotID != nil {
		var v any
		if *p.BotID != "" {
			v = *p.BotID
		}
		args = append(args, v)
		set = append(set, "bot_id = $"+itoa(len(args)))
	}
	addStr("status", p.Status)
	addStr("schedule", p.Schedule)
	if p.MaxAttempts != nil {
		args = append(args, *p.MaxAttempts)
		set = append(set, "max_attempts = $"+itoa(len(args)))
	}
	if len(set) == 0 {
		return s.getCampaign(ctx, tenantID, id)
	}
	q := "UPDATE campaigns SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1 AND id = $2"
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return Campaign{}, err
	}
	if tag.RowsAffected() == 0 {
		return Campaign{}, ErrNotFound
	}
	return s.getCampaign(ctx, tenantID, id)
}

func (s *Store) DeleteCampaign(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM campaigns WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) getCampaign(ctx context.Context, tenantID, id string) (Campaign, error) {
	var c Campaign
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, COALESCE(bot_id, ''), name, status, schedule, lead_count, max_attempts
		FROM campaigns WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&c.ID, &c.TenantID, &c.BotID, &c.Name, &c.Status, &c.Schedule, &c.LeadCount, &c.MaxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// --- Property CRUD ---

func (s *Store) CreateProperty(ctx context.Context, p Property) (Property, error) {
	if p.ID == "" {
		p.ID = newID("prop")
	}
	if p.Requirements == nil {
		p.Requirements = []string{}
	}
	if p.FAQs == nil {
		p.FAQs = []string{}
	}
	reqJSON, _ := json.Marshal(p.Requirements)
	faqJSON, _ := json.Marshal(p.FAQs)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO properties (id, tenant_id, name, address, price, availability, requirements, faqs)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb)`,
		p.ID, p.TenantID, p.Name, p.Address, p.Price, p.Availability, string(reqJSON), string(faqJSON))
	return p, err
}

type PropertyPatch struct {
	Name         *string
	Address      *string
	Price        *string
	Availability *string
	Requirements *[]string
	FAQs         *[]string
}

func (s *Store) UpdateProperty(ctx context.Context, tenantID, id string, p PropertyPatch) error {
	set := []string{}
	args := []any{tenantID, id}
	addStr := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, *val)
		set = append(set, col+" = $"+itoa(len(args)))
	}
	addStr("name", p.Name)
	addStr("address", p.Address)
	addStr("price", p.Price)
	addStr("availability", p.Availability)
	if p.Requirements != nil {
		b, _ := json.Marshal(*p.Requirements)
		args = append(args, string(b))
		set = append(set, "requirements = $"+itoa(len(args))+"::jsonb")
	}
	if p.FAQs != nil {
		b, _ := json.Marshal(*p.FAQs)
		args = append(args, string(b))
		set = append(set, "faqs = $"+itoa(len(args))+"::jsonb")
	}
	if len(set) == 0 {
		return nil
	}
	q := "UPDATE properties SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1 AND id = $2"
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteProperty(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM properties WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- User password change ---

func (s *Store) UpdatePassword(ctx context.Context, userID, hash string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, email, name, role, password_hash, last_login_at, created_at
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.LastLoginAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// itoa is a small stdlib-free int->string helper to keep query builders compact.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var _ = time.Time{}
