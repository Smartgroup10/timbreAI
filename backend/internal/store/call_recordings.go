package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateCallRecording persiste metadata de una grabación recién subida.
// retentionDays > 0 → calculamos retention_due_at = now() + N días para
// que el worker pueda borrarla cuando toque. 0 = conservar indefinido.
//
// Si ya existía una grabación "available" para la misma call, la
// archivamos (status="archived") para no perder el histórico. La nueva
// queda como "available".
func (s *Store) CreateCallRecording(ctx context.Context, in CallRecording, retentionDays int) (CallRecording, error) {
	if in.ID == "" {
		in.ID = newID("rec")
	}
	if in.Status == "" {
		in.Status = "available"
	}
	if in.ContentType == "" {
		in.ContentType = "audio/wav"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return in, err
	}
	defer tx.Rollback(ctx)

	// Archivar grabaciones previas de esta call (no las borramos del
	// storage — quizás el operador quiera comparar versiones; el worker
	// las limpia con la retención normal).
	if _, err := tx.Exec(ctx, `
		UPDATE call_recordings SET status = 'archived', updated_at = now()
		WHERE call_id = $1 AND status = 'available' AND deleted_at IS NULL`,
		in.CallID); err != nil {
		return in, err
	}

	var retentionDue *time.Time
	if retentionDays > 0 {
		t := time.Now().UTC().Add(time.Duration(retentionDays) * 24 * time.Hour)
		retentionDue = &t
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO call_recordings
		  (id, call_id, tenant_id, storage_key, content_type, size_bytes, duration_sec, status, retention_due_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at, retention_due_at`,
		in.ID, in.CallID, in.TenantID, in.StorageKey, in.ContentType, in.SizeBytes, in.DurationSec, in.Status, retentionDue).
		Scan(&in.CreatedAt, &in.UpdatedAt, &in.RetentionDueAt)
	if err != nil {
		return in, err
	}
	return in, tx.Commit(ctx)
}

// GetActiveRecordingForCall devuelve la grabación viva (status=available,
// no soft-deleted) de una call. ErrNotFound si no hay.
func (s *Store) GetActiveRecordingForCall(ctx context.Context, tenantID, callID string) (CallRecording, error) {
	var r CallRecording
	err := s.pool.QueryRow(ctx, `
		SELECT id, call_id, tenant_id, storage_key, content_type, size_bytes, duration_sec,
		       status, deleted_at, retention_due_at, created_at, updated_at
		FROM call_recordings
		WHERE tenant_id = $1 AND call_id = $2 AND status = 'available' AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, tenantID, callID).
		Scan(&r.ID, &r.CallID, &r.TenantID, &r.StorageKey, &r.ContentType, &r.SizeBytes, &r.DurationSec,
			&r.Status, &r.DeletedAt, &r.RetentionDueAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// GetCallRecordingByID resuelve por id con scope tenant. Lo usa el handler
// de DELETE.
func (s *Store) GetCallRecordingByID(ctx context.Context, tenantID, id string) (CallRecording, error) {
	var r CallRecording
	err := s.pool.QueryRow(ctx, `
		SELECT id, call_id, tenant_id, storage_key, content_type, size_bytes, duration_sec,
		       status, deleted_at, retention_due_at, created_at, updated_at
		FROM call_recordings WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&r.ID, &r.CallID, &r.TenantID, &r.StorageKey, &r.ContentType, &r.SizeBytes, &r.DurationSec,
			&r.Status, &r.DeletedAt, &r.RetentionDueAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// CallRecordingListFilter agrupa los parámetros de búsqueda + paginación.
// Limit se capa server-side; el cliente no decide cuánto trae.
type CallRecordingListFilter struct {
	TenantID  string
	Outcome   string // "" = cualquiera
	FromDate  *time.Time
	ToDate    *time.Time
	Limit     int
	Offset    int
}

// ListCallRecordings devuelve grabaciones activas con info de la call
// joinada para evitar N+1 en la UI.
//
// El total se calcula con un count separado — pgx no tiene window functions
// gratis en este path y queremos paginación honesta.
func (s *Store) ListCallRecordings(ctx context.Context, f CallRecordingListFilter) ([]CallRecordingListItem, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	conds := []string{"r.tenant_id = $1", "r.deleted_at IS NULL", "r.status = 'available'"}
	args := []any{f.TenantID}
	if f.Outcome != "" {
		args = append(args, f.Outcome)
		conds = append(conds, "c.outcome = $"+itoa(len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conds = append(conds, "r.created_at >= $"+itoa(len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conds = append(conds, "r.created_at < $"+itoa(len(args)))
	}
	where := "WHERE " + joinAnd(conds)

	// Count total.
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM call_recordings r
		JOIN calls c ON c.id = r.call_id
		`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Page.
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.call_id, r.tenant_id, r.storage_key, r.content_type,
		       r.size_bytes, r.duration_sec, r.status, r.deleted_at, r.retention_due_at,
		       r.created_at, r.updated_at,
		       COALESCE(c.lead_name, ''), c.phone, COALESCE(c.campaign_name, ''), c.outcome
		FROM call_recordings r
		JOIN calls c ON c.id = r.call_id
		`+where+`
		ORDER BY r.created_at DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []CallRecordingListItem{}
	for rows.Next() {
		var i CallRecordingListItem
		if err := rows.Scan(&i.ID, &i.CallID, &i.TenantID, &i.StorageKey, &i.ContentType,
			&i.SizeBytes, &i.DurationSec, &i.Status, &i.DeletedAt, &i.RetentionDueAt,
			&i.CreatedAt, &i.UpdatedAt,
			&i.LeadName, &i.Phone, &i.Campaign, &i.Outcome); err != nil {
			return nil, 0, err
		}
		out = append(out, i)
	}
	return out, total, rows.Err()
}

// SoftDeleteCallRecording marca la fila como borrada. El objeto en MinIO
// lo borra el worker (o el caller, si quiere borrar al momento). Soft
// delete preserva auditoría: "se grabó esta llamada pero se borró por
// petición del usuario el día X".
func (s *Store) SoftDeleteCallRecording(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE call_recordings
		SET deleted_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListExpiredRecordings devuelve grabaciones cuyo retention_due_at ya
// pasó. Las usa el worker de retención para borrarlas. Limit pequeño
// para no traer todo el bucket en una pasada.
func (s *Store) ListExpiredRecordings(ctx context.Context, limit int) ([]CallRecording, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, call_id, tenant_id, storage_key, content_type, size_bytes, duration_sec,
		       status, deleted_at, retention_due_at, created_at, updated_at
		FROM call_recordings
		WHERE deleted_at IS NULL
		  AND retention_due_at IS NOT NULL
		  AND retention_due_at <= now()
		ORDER BY retention_due_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CallRecording{}
	for rows.Next() {
		var r CallRecording
		if err := rows.Scan(&r.ID, &r.CallID, &r.TenantID, &r.StorageKey, &r.ContentType,
			&r.SizeBytes, &r.DurationSec, &r.Status, &r.DeletedAt, &r.RetentionDueAt,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TenantRecordingUsage es el agregado de almacenamiento usado por un
// tenant para mostrar en analytics ("3.2 GB").
type TenantRecordingUsage struct {
	TotalBytes int64 `json:"totalBytes"`
	Count      int   `json:"count"`
	OldestAt   *time.Time `json:"oldestAt,omitempty"`
}

// TenantRecordingUsage devuelve bytes acumulados y cuenta de grabaciones
// activas. Lo usa el dashboard.
func (s *Store) TenantRecordingUsage(ctx context.Context, tenantID string) (TenantRecordingUsage, error) {
	var u TenantRecordingUsage
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0), count(*), MIN(created_at)
		FROM call_recordings
		WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).
		Scan(&u.TotalBytes, &u.Count, &u.OldestAt)
	return u, err
}

func joinAnd(parts []string) string {
	if len(parts) == 0 {
		return "1=1"
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += " AND " + p
	}
	return out
}
