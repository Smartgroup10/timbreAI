package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ListUsersByTenant returns all users scoped to a tenant (excluding platform admins, which have
// no tenant). Used by the per-tenant team management UI.
func (s *Store) ListUsersByTenant(ctx context.Context, tenantID string) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, email, name, role, last_login_at, created_at
		FROM users WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &u.LastLoginAt, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type CreateUserInput struct {
	TenantID     string
	Email        string
	Name         string
	Role         string
	PasswordHash string
}

func (s *Store) InsertTenantUser(ctx context.Context, in CreateUserInput) (User, error) {
	id := newID("usr")
	email := strings.ToLower(strings.TrimSpace(in.Email))
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, name, role, password_hash)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, in.TenantID, email, in.Name, in.Role, in.PasswordHash)
	if err != nil {
		return User{}, err
	}
	return s.GetUser(ctx, id)
}

func (s *Store) UpdateTenantUserRole(ctx context.Context, tenantID, userID, role string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET role = $3 WHERE tenant_id = $1 AND id = $2`,
		tenantID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1 AND id = $2`,
		tenantID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EmailTaken returns true if the email already exists in any tenant (since email is globally unique).
func (s *Store) EmailTaken(ctx context.Context, email string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(email) = lower($1)`, email).Scan(&n)
	return n > 0, err
}

// --- Transcripts (used by the call detail page and the voice-agent webhook) ---

func (s *Store) AppendTranscript(ctx context.Context, tenantID, callID, role, text string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO transcripts (id, tenant_id, call_id, role, text)
		VALUES ($1, $2, $3, $4, $5)`,
		newID("tr"), tenantID, callID, role, text)
	return err
}

type Transcript struct {
	ID         string `json:"id"`
	CallID     string `json:"callId"`
	Role       string `json:"role"`
	Text       string `json:"text"`
	OccurredAt string `json:"occurredAt"`
}

func (s *Store) ListCallTranscripts(ctx context.Context, tenantID, callID string) ([]Transcript, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, call_id, role, text, to_char(occurred_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
		FROM transcripts WHERE tenant_id = $1 AND call_id = $2
		ORDER BY occurred_at`, tenantID, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transcript{}
	for rows.Next() {
		var t Transcript
		if err := rows.Scan(&t.ID, &t.CallID, &t.Role, &t.Text, &t.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindCallByVoiceSession looks up the parent call from a voice-agent session id. The webhook uses
// this to validate that the incoming session belongs to a real call before persisting transcripts.
func (s *Store) FindCallByVoiceSession(ctx context.Context, sessionID string) (Call, error) {
	var c Call
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome,
		       duration_sec, channel_id, voice_session_id, started_at, ended_at, summary, recording_url, provider
		FROM calls WHERE voice_session_id = $1`, sessionID).
		Scan(&c.ID, &c.TenantID, &c.LeadID, &c.CampaignID, &c.LeadName, &c.Campaign, &c.Phone, &c.Status, &c.Outcome,
			&c.DurationSec, &c.ChannelID, &c.VoiceSessionID, &c.StartedAt, &c.EndedAt, &c.Summary, &c.RecordingURL, &c.Provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// UpdateCallAMD persiste el veredicto del detector AMD para una llamada.
// El voice-agent lo reporta una vez por sesión. voicemailDropped es true
// si el bot llegó a soltar el mensaje pre-grabado al buzón (futuro: hoy
// se marca preventivamente al detectar machine cuando action=drop_message).
func (s *Store) UpdateCallAMD(ctx context.Context, callID, result string, voicemailDropped bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE calls SET amd_result = $2, voicemail_dropped = $3
		WHERE id = $1`, callID, result, voicemailDropped)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
