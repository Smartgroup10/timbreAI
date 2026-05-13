package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// --- SIP trunks (platform-level) ---

func (s *Store) ListTrunks(ctx context.Context) ([]SIPTrunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.provider, t.asterisk_endpoint, t.host, t.port, t.status, t.notes, t.created_at,
		       (SELECT count(*) FROM dids d WHERE d.trunk_id = t.id) AS did_count
		FROM sip_trunks t
		ORDER BY t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SIPTrunk{}
	for rows.Next() {
		var t SIPTrunk
		if err := rows.Scan(&t.ID, &t.Name, &t.Provider, &t.AsteriskEndpoint, &t.Host, &t.Port, &t.Status, &t.Notes, &t.CreatedAt, &t.DIDCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTrunk(ctx context.Context, id string) (SIPTrunk, error) {
	var t SIPTrunk
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, provider, asterisk_endpoint, host, port, status, notes, created_at, 0
		FROM sip_trunks WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Provider, &t.AsteriskEndpoint, &t.Host, &t.Port, &t.Status, &t.Notes, &t.CreatedAt, &t.DIDCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (s *Store) CreateTrunk(ctx context.Context, t SIPTrunk) (SIPTrunk, error) {
	if t.ID == "" {
		t.ID = newID("trunk")
	}
	if t.Status == "" {
		t.Status = "active"
	}
	if t.Port == 0 {
		t.Port = 5060
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sip_trunks (id, name, provider, asterisk_endpoint, host, port, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		t.ID, t.Name, t.Provider, t.AsteriskEndpoint, t.Host, t.Port, t.Status, t.Notes)
	return t, err
}

func (s *Store) UpdateTrunk(ctx context.Context, t SIPTrunk) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sip_trunks SET name = $2, provider = $3, asterisk_endpoint = $4, host = $5, port = $6,
		                     status = $7, notes = $8
		WHERE id = $1`,
		t.ID, t.Name, t.Provider, t.AsteriskEndpoint, t.Host, t.Port, t.Status, t.Notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTrunk(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sip_trunks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- DIDs ---

// ListDIDs returns DIDs scoped to a tenant (tenantID != "") or all DIDs (tenantID == "").
// When called by platform admins with no scope, pass "" to see the global pool.
func (s *Store) ListDIDs(ctx context.Context, tenantID string) ([]DID, error) {
	var rows pgx.Rows
	var err error
	if tenantID == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT d.id, d.trunk_id, t.name, t.asterisk_endpoint, d.tenant_id, d.e164, d.label, d.status, d.created_at
			FROM dids d JOIN sip_trunks t ON t.id = d.trunk_id
			ORDER BY d.e164`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT d.id, d.trunk_id, t.name, t.asterisk_endpoint, d.tenant_id, d.e164, d.label, d.status, d.created_at
			FROM dids d JOIN sip_trunks t ON t.id = d.trunk_id
			WHERE d.tenant_id = $1
			ORDER BY d.e164`, tenantID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DID{}
	for rows.Next() {
		var d DID
		if err := rows.Scan(&d.ID, &d.TrunkID, &d.TrunkName, &d.AsteriskEndpoint, &d.TenantID, &d.E164, &d.Label, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDID(ctx context.Context, id string) (DID, error) {
	var d DID
	err := s.pool.QueryRow(ctx, `
		SELECT d.id, d.trunk_id, t.name, t.asterisk_endpoint, d.tenant_id, d.e164, d.label, d.status, d.created_at
		FROM dids d JOIN sip_trunks t ON t.id = d.trunk_id
		WHERE d.id = $1`, id).
		Scan(&d.ID, &d.TrunkID, &d.TrunkName, &d.AsteriskEndpoint, &d.TenantID, &d.E164, &d.Label, &d.Status, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) CreateDID(ctx context.Context, d DID) (DID, error) {
	if d.ID == "" {
		d.ID = newID("did")
	}
	if d.Status == "" {
		d.Status = "active"
	}
	var tenantID any
	if d.TenantID != nil && *d.TenantID != "" {
		tenantID = *d.TenantID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO dids (id, trunk_id, tenant_id, e164, label, status)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		d.ID, d.TrunkID, tenantID, d.E164, d.Label, d.Status)
	if err != nil {
		return d, err
	}
	return s.GetDID(ctx, d.ID)
}

// AssignDIDToTenant attaches/detaches a DID. Pass nil tenantID to release the DID back to the pool.
// Also clears any bot->did association when the tenant changes.
func (s *Store) AssignDIDToTenant(ctx context.Context, didID string, tenantID *string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current *string
	err = tx.QueryRow(ctx, `SELECT tenant_id FROM dids WHERE id = $1`, didID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if current != nil && (tenantID == nil || *tenantID == "" || *current != *tenantID) {
		// DID is moving away from the current tenant: detach it from any bot of that tenant.
		if _, err := tx.Exec(ctx, `UPDATE bots SET did_id = NULL WHERE did_id = $1`, didID); err != nil {
			return err
		}
	}

	var newTenant any
	if tenantID != nil && *tenantID != "" {
		newTenant = *tenantID
	}
	if _, err := tx.Exec(ctx, `UPDATE dids SET tenant_id = $2 WHERE id = $1`, didID, newTenant); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateDID(ctx context.Context, d DID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE dids SET e164 = $2, label = $3, status = $4
		WHERE id = $1`, d.ID, d.E164, d.Label, d.Status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteDID(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM dids WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LookupDIDForBot returns the DID assigned to a bot (with its trunk endpoint), or ErrNotFound.
func (s *Store) LookupDIDForBot(ctx context.Context, tenantID, botID string) (DID, error) {
	var d DID
	err := s.pool.QueryRow(ctx, `
		SELECT d.id, d.trunk_id, t.name, t.asterisk_endpoint, d.tenant_id, d.e164, d.label, d.status, d.created_at
		FROM bots b
		JOIN dids d ON d.id = b.did_id
		JOIN sip_trunks t ON t.id = d.trunk_id
		WHERE b.tenant_id = $1 AND b.id = $2`, tenantID, botID).
		Scan(&d.ID, &d.TrunkID, &d.TrunkName, &d.AsteriskEndpoint, &d.TenantID, &d.E164, &d.Label, &d.Status, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}
