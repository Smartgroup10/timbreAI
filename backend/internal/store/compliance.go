package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// --- Do Not Call ---

func (s *Store) ListDoNotCall(ctx context.Context, tenantID string) ([]DoNotCallEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, phone, reason, created_at
		FROM do_not_call WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DoNotCallEntry{}
	for rows.Next() {
		var e DoNotCallEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Phone, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) AddDoNotCall(ctx context.Context, e DoNotCallEntry) (DoNotCallEntry, error) {
	if e.ID == "" {
		e.ID = newID("dnc")
	}
	e.Phone = strings.TrimSpace(e.Phone)
	if e.Reason == "" {
		e.Reason = "manual"
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO do_not_call (id, tenant_id, phone, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, phone) DO UPDATE SET reason = EXCLUDED.reason
		RETURNING id, created_at`,
		e.ID, e.TenantID, e.Phone, e.Reason).Scan(&e.ID, &e.CreatedAt)
	return e, err
}

func (s *Store) RemoveDoNotCall(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM do_not_call WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Audit log ---

type AuditEvent struct {
	TenantID   string
	ActorID    string
	Action     string
	EntityType string
	EntityID   string
	Payload    map[string]any
}

func (s *Store) WriteAudit(ctx context.Context, e AuditEvent) {
	payload, _ := json.Marshal(e.Payload)
	if payload == nil {
		payload = []byte("{}")
	}
	var tenantID any
	if e.TenantID != "" {
		tenantID = e.TenantID
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, tenant_id, actor_id, action, entity_type, entity_id, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
		newID("audit"), tenantID, e.ActorID, e.Action, e.EntityType, e.EntityID, string(payload))
}

func (s *Store) ListAudit(ctx context.Context, tenantID string, limit int) ([]AuditLogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows pgx.Rows
	var err error
	if tenantID == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT a.id, a.tenant_id, COALESCE(a.actor_id, ''), COALESCE(u.email, ''),
			       a.action, a.entity_type, a.entity_id, a.payload, a.created_at
			FROM audit_logs a
			LEFT JOIN users u ON u.id = a.actor_id
			ORDER BY a.created_at DESC
			LIMIT $1`, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT a.id, a.tenant_id, COALESCE(a.actor_id, ''), COALESCE(u.email, ''),
			       a.action, a.entity_type, a.entity_id, a.payload, a.created_at
			FROM audit_logs a
			LEFT JOIN users u ON u.id = a.actor_id
			WHERE a.tenant_id = $1
			ORDER BY a.created_at DESC
			LIMIT $2`, tenantID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditLogEntry{}
	for rows.Next() {
		var e AuditLogEntry
		var payload []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.ActorEmail,
			&e.Action, &e.EntityType, &e.EntityID, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &e.Payload)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

var _ = errors.New
