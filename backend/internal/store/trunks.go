package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SIP trunks viven en DOS sitios:
//   - sip_trunks: tabla de metadata interna del portal (id, provider, notes, etc.)
//   - ps_endpoints / ps_auths / ps_aors / ps_registrations / ps_endpoint_id_ips:
//     schema canónico de Asterisk Realtime PJSIP. Asterisk lee estas tablas en
//     vivo (sin reload) vía sorcery + res_config_pgsql.
//
// Cada operación (Create/Update/Delete) escribe en TRANSACCIÓN sobre ambos lados
// para evitar quedar desincronizado. Las filas ps_* se identifican por nombre:
//
//   ps_endpoints.id        = sip_trunks.asterisk_endpoint
//   ps_auths.id            = <endpoint>-auth
//   ps_aors.id             = <endpoint>
//   ps_registrations.id    = <endpoint>-reg     (solo si register_required)
//   ps_endpoint_id_ips.id  = <endpoint>-identify (solo si identify_ip != "")

func (s *Store) ListTrunks(ctx context.Context) ([]SIPTrunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.provider, t.asterisk_endpoint, t.host, t.port,
		       t.sip_username, t.sip_password, t.register_required, t.identify_ip,
		       t.status, t.notes, t.created_at,
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
		if err := rows.Scan(&t.ID, &t.Name, &t.Provider, &t.AsteriskEndpoint, &t.Host, &t.Port,
			&t.Username, &t.Password, &t.Register, &t.IdentifyIP,
			&t.Status, &t.Notes, &t.CreatedAt, &t.DIDCount); err != nil {
			return nil, err
		}
		// No leakeamos el password al frontend en list.
		if t.Password != "" {
			t.Password = "********"
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTrunk(ctx context.Context, id string) (SIPTrunk, error) {
	var t SIPTrunk
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, provider, asterisk_endpoint, host, port,
		       sip_username, sip_password, register_required, identify_ip,
		       status, notes, created_at, 0
		FROM sip_trunks WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Provider, &t.AsteriskEndpoint, &t.Host, &t.Port,
			&t.Username, &t.Password, &t.Register, &t.IdentifyIP,
			&t.Status, &t.Notes, &t.CreatedAt, &t.DIDCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if t.Password != "" {
		t.Password = "********"
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
	if t.AsteriskEndpoint == "" {
		return t, errors.New("asterisk_endpoint_required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return t, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO sip_trunks (id, name, provider, asterisk_endpoint, host, port,
		                       sip_username, sip_password, register_required, identify_ip,
		                       status, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.Name, t.Provider, t.AsteriskEndpoint, t.Host, t.Port,
		t.Username, t.Password, t.Register, t.IdentifyIP,
		t.Status, t.Notes); err != nil {
		return t, err
	}
	if err := upsertRealtimeTrunk(ctx, tx, t); err != nil {
		return t, err
	}
	if err := tx.Commit(ctx); err != nil {
		return t, err
	}
	// Devolvemos con password enmascarado.
	if t.Password != "" {
		t.Password = "********"
	}
	return t, nil
}

func (s *Store) UpdateTrunk(ctx context.Context, t SIPTrunk) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Si el frontend manda el password enmascarado, mantenemos el actual.
	var currentPassword string
	if err := tx.QueryRow(ctx, `SELECT sip_password FROM sip_trunks WHERE id = $1`, t.ID).Scan(&currentPassword); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if t.Password == "" || t.Password == "********" {
		t.Password = currentPassword
	}

	tag, err := tx.Exec(ctx, `
		UPDATE sip_trunks SET name=$2, provider=$3, asterisk_endpoint=$4, host=$5, port=$6,
		                     sip_username=$7, sip_password=$8, register_required=$9, identify_ip=$10,
		                     status=$11, notes=$12
		WHERE id = $1`,
		t.ID, t.Name, t.Provider, t.AsteriskEndpoint, t.Host, t.Port,
		t.Username, t.Password, t.Register, t.IdentifyIP,
		t.Status, t.Notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	// Borramos y recreamos las filas ps_*. Es más simple que UPDATE selectivo
	// y como Asterisk lee on-demand no hay race observable.
	if err := deleteRealtimeTrunk(ctx, tx, t.AsteriskEndpoint); err != nil {
		return err
	}
	if err := upsertRealtimeTrunk(ctx, tx, t); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteTrunk(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var endpoint string
	if err := tx.QueryRow(ctx, `SELECT asterisk_endpoint FROM sip_trunks WHERE id = $1`, id).Scan(&endpoint); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM sip_trunks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := deleteRealtimeTrunk(ctx, tx, endpoint); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// upsertRealtimeTrunk inserta las filas ps_* correspondientes a un trunk.
// Asume que cualquier fila previa con los mismos ids ya fue eliminada (lo
// hacemos en UpdateTrunk explícitamente).
func upsertRealtimeTrunk(ctx context.Context, tx pgx.Tx, t SIPTrunk) error {
	if t.AsteriskEndpoint == "" {
		return errors.New("asterisk_endpoint_required")
	}
	endpoint := t.AsteriskEndpoint
	authID := endpoint + "-auth"
	aorID := endpoint
	regID := endpoint + "-reg"
	identifyID := endpoint + "-identify"

	// AOR: contacto del proveedor.
	contact := ""
	if t.Host != "" {
		port := t.Port
		if port == 0 {
			port = 5060
		}
		contact = fmt.Sprintf("sip:%s:%d", t.Host, port)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ps_aors (id, contact, max_contacts, qualify_frequency)
		VALUES ($1, NULLIF($2,''), 1, 60)`, aorID, contact); err != nil {
		return fmt.Errorf("ps_aors insert: %w", err)
	}

	// Auth: credenciales.
	if t.Username != "" || t.Password != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ps_auths (id, auth_type, username, password)
			VALUES ($1, 'userpass', $2, $3)`, authID, t.Username, t.Password); err != nil {
			return fmt.Errorf("ps_auths insert: %w", err)
		}
	}

	// Endpoint propiamente dicho.
	var outboundAuth any
	if t.Username != "" || t.Password != "" {
		outboundAuth = authID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ps_endpoints (id, transport, aors, outbound_auth, context, disallow, allow,
		                         direct_media, from_user, from_domain, rtp_symmetric,
		                         force_rport, rewrite_contact)
		VALUES ($1, 'transport-udp', $2, $3, 'from-trunk', 'all', 'ulaw,alaw',
		        'no', NULLIF($4,''), NULLIF($5,''), 'yes',
		        'yes', 'yes')`,
		endpoint, aorID, outboundAuth, t.Username, t.Host); err != nil {
		return fmt.Errorf("ps_endpoints insert: %w", err)
	}

	// Registration (solo si el trunk lo requiere — Twilio/Vonage típicamente sí).
	if t.Register && t.Host != "" && t.Username != "" {
		port := t.Port
		if port == 0 {
			port = 5060
		}
		serverURI := fmt.Sprintf("sip:%s:%d", t.Host, port)
		clientURI := fmt.Sprintf("sip:%s@%s", t.Username, t.Host)
		if _, err := tx.Exec(ctx, `
			INSERT INTO ps_registrations (id, outbound_auth, server_uri, client_uri,
			                              retry_interval, expiration, transport)
			VALUES ($1, $2, $3, $4, 60, 3600, 'transport-udp')`,
			regID, authID, serverURI, clientURI); err != nil {
			return fmt.Errorf("ps_registrations insert: %w", err)
		}
	}

	// Identify por IP (Twilio Elastic Trunking, sin REGISTER).
	if t.IdentifyIP != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ps_endpoint_id_ips (id, endpoint, match)
			VALUES ($1, $2, $3)`, identifyID, endpoint, t.IdentifyIP); err != nil {
			return fmt.Errorf("ps_endpoint_id_ips insert: %w", err)
		}
	}
	return nil
}

func deleteRealtimeTrunk(ctx context.Context, tx pgx.Tx, endpoint string) error {
	if endpoint == "" {
		return nil
	}
	authID := endpoint + "-auth"
	regID := endpoint + "-reg"
	identifyID := endpoint + "-identify"
	for _, q := range []struct {
		sql string
		arg string
	}{
		{`DELETE FROM ps_endpoint_id_ips WHERE id = $1`, identifyID},
		{`DELETE FROM ps_registrations   WHERE id = $1`, regID},
		{`DELETE FROM ps_endpoints       WHERE id = $1`, endpoint},
		{`DELETE FROM ps_auths           WHERE id = $1`, authID},
		{`DELETE FROM ps_aors            WHERE id = $1`, endpoint},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.arg); err != nil {
			return fmt.Errorf("delete realtime row: %w", err)
		}
	}
	return nil
}

// --- DIDs (sin cambios) ---

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
