package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateScheduledMeeting persiste una cita después de que se haya
// creado con éxito en el provider. La call que la generó queda
// enlazada en created_call_id para auditoría.
func (s *Store) CreateScheduledMeeting(ctx context.Context, m ScheduledMeeting) (ScheduledMeeting, error) {
	if m.ID == "" {
		m.ID = newID("mtg")
	}
	if m.Provider == "" {
		m.Provider = "google"
	}
	if m.CalendarID == "" {
		m.CalendarID = "primary"
	}
	if m.Status == "" {
		m.Status = "scheduled"
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO scheduled_meetings
		  (id, tenant_id, bot_id, lead_id, lead_phone, provider, provider_event_id,
		   calendar_id, html_link, title, start_at, end_at, attendee_email, status, created_call_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING created_at, updated_at`,
		m.ID, m.TenantID, m.BotID, m.LeadID, m.LeadPhone, m.Provider, m.ProviderEventID,
		m.CalendarID, m.HTMLLink, m.Title, m.StartAt, m.EndAt, m.AttendeeEmail, m.Status, m.CreatedCallID).
		Scan(&m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// ListScheduledMeetingsForLead devuelve las citas activas (status=scheduled)
// que pertenecen al lead identificado por lead_id O por lead_phone, dentro
// del tenant. Esta es la lookup de ownership que usa el bot al listar
// "tus citas". OR lógico cubre el caso "el lead nos llama desde un
// número que tenemos en BD pero su `leads.id` no está en el contexto
// de la call actual".
//
// Solo futuro y presente — no mostramos citas pasadas (start_at >= now()),
// porque la tool sirve para "qué tengo agendado".
func (s *Store) ListScheduledMeetingsForLead(ctx context.Context, tenantID, leadID, leadPhone string) ([]ScheduledMeeting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, bot_id, lead_id, lead_phone, provider, provider_event_id,
		       calendar_id, html_link, title, start_at, end_at, attendee_email, status,
		       created_call_id, created_at, updated_at
		FROM scheduled_meetings
		WHERE tenant_id = $1
		  AND status = 'scheduled'
		  AND end_at >= now()
		  AND (
		    ($2 <> '' AND lead_id = $2)
		    OR ($3 <> '' AND lead_phone = $3)
		  )
		ORDER BY start_at`, tenantID, leadID, leadPhone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScheduledMeeting{}
	for rows.Next() {
		var m ScheduledMeeting
		if err := rows.Scan(&m.ID, &m.TenantID, &m.BotID, &m.LeadID, &m.LeadPhone,
			&m.Provider, &m.ProviderEventID, &m.CalendarID, &m.HTMLLink, &m.Title,
			&m.StartAt, &m.EndAt, &m.AttendeeEmail, &m.Status, &m.CreatedCallID,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetScheduledMeetingForLead busca UNA cita por id pero VALIDANDO que
// pertenezca al lead (por lead_id o lead_phone). Si el id existe pero
// pertenece a otro lead, devolvemos ErrNotFound — no revelamos que
// existe pero no es suya.
//
// Esta es la función crítica de seguridad: cancel/reschedule llaman
// aquí antes de tocar Google.
func (s *Store) GetScheduledMeetingForLead(ctx context.Context, tenantID, meetingID, leadID, leadPhone string) (ScheduledMeeting, error) {
	var m ScheduledMeeting
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, bot_id, lead_id, lead_phone, provider, provider_event_id,
		       calendar_id, html_link, title, start_at, end_at, attendee_email, status,
		       created_call_id, created_at, updated_at
		FROM scheduled_meetings
		WHERE tenant_id = $1 AND id = $2 AND status = 'scheduled'
		  AND (
		    ($3 <> '' AND lead_id = $3)
		    OR ($4 <> '' AND lead_phone = $4)
		  )`,
		tenantID, meetingID, leadID, leadPhone).
		Scan(&m.ID, &m.TenantID, &m.BotID, &m.LeadID, &m.LeadPhone,
			&m.Provider, &m.ProviderEventID, &m.CalendarID, &m.HTMLLink, &m.Title,
			&m.StartAt, &m.EndAt, &m.AttendeeEmail, &m.Status, &m.CreatedCallID,
			&m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// MarkScheduledMeetingCancelled idempotente — si ya está cancelada
// devuelve sin error. El borrado en Google va por otro lado (calendar
// service); aquí solo trackeamos.
func (s *Store) MarkScheduledMeetingCancelled(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scheduled_meetings
		SET status = 'cancelled', updated_at = now()
		WHERE id = $1 AND status = 'scheduled'`, id)
	return err
}

// UpdateScheduledMeetingTimes actualiza start/end después de un
// reschedule exitoso en Google. Si la fila no existe, ErrNotFound.
func (s *Store) UpdateScheduledMeetingTimes(ctx context.Context, id string, startAt, endAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE scheduled_meetings
		SET start_at = $2, end_at = $3, updated_at = now()
		WHERE id = $1 AND status = 'scheduled'`, id, startAt, endAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
