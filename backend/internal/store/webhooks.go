package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// generateSecret produce 32 bytes random hex — entropía suficiente para
// que el receptor pueda verificar HMAC sin que se adivine. Generado en
// backend (no en cliente) para evitar problemas con randomness del navegador.
func generateSecret() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (s *Store) ListWebhookEndpoints(ctx context.Context, tenantID string) ([]WebhookEndpoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, url, events, active, created_at, updated_at
		FROM webhook_endpoints
		WHERE tenant_id = $1
		ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebhookEndpoint{}
	for rows.Next() {
		var e WebhookEndpoint
		var events []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Name, &e.URL, &events, &e.Active, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(events, &e.Events)
		// secret NO se devuelve en list.
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListWebhookEndpointsForEvent filtra los endpoints suscritos a un tipo
// concreto. Lo usa el dispatcher para evitar hacer N requests cuando un
// tenant tiene un endpoint solo para call.qualified y el evento es otro.
func (s *Store) ListWebhookEndpointsForEvent(ctx context.Context, tenantID, eventType string) ([]WebhookEndpoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, url, secret, events, active, created_at, updated_at
		FROM webhook_endpoints
		WHERE tenant_id = $1 AND active = true
		  AND events @> $2::jsonb`, tenantID, `["`+eventType+`"]`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebhookEndpoint{}
	for rows.Next() {
		var e WebhookEndpoint
		var events []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Name, &e.URL, &e.Secret, &events, &e.Active, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(events, &e.Events)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CreateWebhookEndpoint(ctx context.Context, e WebhookEndpoint) (WebhookEndpoint, error) {
	if e.ID == "" {
		e.ID = newID("webhook")
	}
	if e.Secret == "" {
		e.Secret = generateSecret()
	}
	if e.Events == nil {
		e.Events = []string{}
	}
	events, _ := json.Marshal(e.Events)
	err := s.pool.QueryRow(ctx, `
		INSERT INTO webhook_endpoints (id, tenant_id, name, url, secret, events, active)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
		RETURNING created_at, updated_at`,
		e.ID, e.TenantID, e.Name, e.URL, e.Secret, events, e.Active).
		Scan(&e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (s *Store) UpdateWebhookEndpoint(ctx context.Context, e WebhookEndpoint) (WebhookEndpoint, error) {
	events, _ := json.Marshal(e.Events)
	tag, err := s.pool.Exec(ctx, `
		UPDATE webhook_endpoints
		SET name = $3, url = $4, events = $5::jsonb, active = $6, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		e.TenantID, e.ID, e.Name, e.URL, events, e.Active)
	if err != nil {
		return e, err
	}
	if tag.RowsAffected() == 0 {
		return e, ErrNotFound
	}
	return s.GetWebhookEndpoint(ctx, e.TenantID, e.ID)
}

// RegenerateWebhookSecret rota el secret. Útil cuando el operador
// sospecha que se filtró. Devuelve el nuevo secret una sola vez.
func (s *Store) RegenerateWebhookSecret(ctx context.Context, tenantID, id string) (string, error) {
	secret := generateSecret()
	tag, err := s.pool.Exec(ctx, `
		UPDATE webhook_endpoints SET secret = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, secret)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return secret, nil
}

func (s *Store) GetWebhookEndpoint(ctx context.Context, tenantID, id string) (WebhookEndpoint, error) {
	var e WebhookEndpoint
	var events []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, url, events, active, created_at, updated_at
		FROM webhook_endpoints WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&e.ID, &e.TenantID, &e.Name, &e.URL, &events, &e.Active, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	_ = json.Unmarshal(events, &e.Events)
	return e, nil
}

func (s *Store) DeleteWebhookEndpoint(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM webhook_endpoints WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) LogWebhookDelivery(ctx context.Context, d WebhookDelivery) error {
	if d.ID == "" {
		d.ID = newID("deliv")
	}
	payload, _ := json.Marshal(d.Payload)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, tenant_id, endpoint_id, event_type, payload, status_code, error, attempt, delivered_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9)`,
		d.ID, d.TenantID, d.EndpointID, d.EventType, payload, d.StatusCode, d.Error, d.Attempt, d.DeliveredAt)
	return err
}

// ListWebhookDeliveries devuelve las últimas 50 entregas (cualquier estado)
// para mostrar histórico en la UI de webhooks.
func (s *Store) ListWebhookDeliveries(ctx context.Context, tenantID string, limit int) ([]WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, endpoint_id, event_type, payload, status_code, error, attempt, delivered_at, created_at
		FROM webhook_deliveries
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebhookDelivery{}
	for rows.Next() {
		var d WebhookDelivery
		var payload []byte
		if err := rows.Scan(&d.ID, &d.TenantID, &d.EndpointID, &d.EventType, &payload,
			&d.StatusCode, &d.Error, &d.Attempt, &d.DeliveredAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &d.Payload)
		out = append(out, d)
	}
	return out, rows.Err()
}
