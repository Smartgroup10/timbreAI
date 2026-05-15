package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"timbre/backend/internal/secretcrypto"
)

// UpsertBotCalendarIntegration crea o reemplaza la integración OAuth de
// un bot con un calendar provider. Lo usamos en el callback del flow
// para no tener un Create + Update separado — si el operador re-conecta
// (rota tokens), simplemente sobreescribimos.
//
// refreshToken/accessToken vienen en claro; los ciframos aquí con
// secretsKey antes de persistir. Lo opuesto a esto: ReadBotCalendarIntegration
// los descifra al leer.
func (s *Store) UpsertBotCalendarIntegration(ctx context.Context, in BotCalendarIntegration) (BotCalendarIntegration, error) {
	if in.ID == "" {
		in.ID = newID("cal")
	}
	if in.Provider == "" {
		in.Provider = "google"
	}
	if in.CalendarID == "" {
		in.CalendarID = "primary"
	}
	refreshEnc, err := secretcrypto.Encrypt(s.secretsKey, []byte(in.RefreshTokenPlain))
	if err != nil {
		return in, err
	}
	var accessEnc []byte
	if in.AccessTokenPlain != "" {
		ct, err := secretcrypto.Encrypt(s.secretsKey, []byte(in.AccessTokenPlain))
		if err != nil {
			return in, err
		}
		accessEnc = ct
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO bot_calendar_integrations
		  (id, tenant_id, bot_id, provider, account_email, calendar_id,
		   refresh_token_encrypted, access_token_encrypted, access_token_expires_at,
		   scopes, connected_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now())
		ON CONFLICT (bot_id, provider) DO UPDATE SET
		  account_email           = EXCLUDED.account_email,
		  calendar_id             = EXCLUDED.calendar_id,
		  refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
		  access_token_encrypted  = EXCLUDED.access_token_encrypted,
		  access_token_expires_at = EXCLUDED.access_token_expires_at,
		  scopes                  = EXCLUDED.scopes,
		  updated_at              = now()
		RETURNING id, connected_at, updated_at`,
		in.ID, in.TenantID, in.BotID, in.Provider, in.AccountEmail, in.CalendarID,
		refreshEnc, accessEnc, in.AccessTokenExpiresAt, in.Scopes).
		Scan(&in.ID, &in.ConnectedAt, &in.UpdatedAt)
	return in, err
}

// GetBotCalendarIntegration devuelve la integración con los tokens en
// claro descifrados — solo lo llaman los handlers internos que van a
// hablar con Google.
func (s *Store) GetBotCalendarIntegration(ctx context.Context, tenantID, botID, provider string) (BotCalendarIntegration, error) {
	if provider == "" {
		provider = "google"
	}
	var (
		out      BotCalendarIntegration
		refEnc   []byte
		accEnc   []byte
		lastUsed *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, bot_id, provider, account_email, calendar_id,
		       refresh_token_encrypted, access_token_encrypted, access_token_expires_at,
		       scopes, connected_at, last_used_at, updated_at
		FROM bot_calendar_integrations
		WHERE tenant_id = $1 AND bot_id = $2 AND provider = $3`,
		tenantID, botID, provider).
		Scan(&out.ID, &out.TenantID, &out.BotID, &out.Provider, &out.AccountEmail, &out.CalendarID,
			&refEnc, &accEnc, &out.AccessTokenExpiresAt,
			&out.Scopes, &out.ConnectedAt, &lastUsed, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.LastUsedAt = lastUsed

	refreshPlain, err := secretcrypto.Decrypt(s.secretsKey, refEnc)
	if err != nil {
		return out, err
	}
	out.RefreshTokenPlain = string(refreshPlain)
	if len(accEnc) > 0 {
		access, err := secretcrypto.Decrypt(s.secretsKey, accEnc)
		if err != nil {
			return out, err
		}
		out.AccessTokenPlain = string(access)
	}
	return out, nil
}

// ListBotCalendarIntegrationsForBot devuelve qué providers tiene
// conectado un bot (vista UI). Sin tokens — solo metadata.
func (s *Store) ListBotCalendarIntegrationsForBot(ctx context.Context, tenantID, botID string) ([]BotCalendarIntegration, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, bot_id, provider, account_email, calendar_id,
		       scopes, connected_at, last_used_at, updated_at
		FROM bot_calendar_integrations
		WHERE tenant_id = $1 AND bot_id = $2
		ORDER BY connected_at`, tenantID, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotCalendarIntegration{}
	for rows.Next() {
		var c BotCalendarIntegration
		var lastUsed *time.Time
		if err := rows.Scan(&c.ID, &c.TenantID, &c.BotID, &c.Provider, &c.AccountEmail, &c.CalendarID,
			&c.Scopes, &c.ConnectedAt, &lastUsed, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.LastUsedAt = lastUsed
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteBotCalendarIntegration desconecta el bot del provider. Idempotente
// — un DELETE sobre una integración inexistente devuelve nil, no error,
// para que el endpoint de "desconectar" sea retry-friendly.
func (s *Store) DeleteBotCalendarIntegration(ctx context.Context, tenantID, botID, provider string) error {
	if provider == "" {
		provider = "google"
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM bot_calendar_integrations
		WHERE tenant_id = $1 AND bot_id = $2 AND provider = $3`,
		tenantID, botID, provider)
	return err
}

// UpdateBotCalendarAccessToken renueva el access token después de un
// refresh. Lo llamamos cuando el access expira; el refresh sigue válido.
func (s *Store) UpdateBotCalendarAccessToken(ctx context.Context, id, accessTokenPlain string, expiresAt time.Time) error {
	enc, err := secretcrypto.Encrypt(s.secretsKey, []byte(accessTokenPlain))
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE bot_calendar_integrations
		SET access_token_encrypted = $2, access_token_expires_at = $3,
		    last_used_at = now(), updated_at = now()
		WHERE id = $1`, id, enc, expiresAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
