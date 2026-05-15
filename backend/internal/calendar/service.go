package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"timbre/backend/internal/store"
)

// Service envuelve el flow OAuth + las operaciones sobre Google
// Calendar, usando el store como persistencia. Cualquier código que
// quiera tocar el calendar de un bot pasa por aquí — no expone tokens.
type Service struct {
	cfg    OAuthConfig
	store  *store.Store
	logger *slog.Logger
}

func NewService(cfg OAuthConfig, st *store.Store, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, store: st, logger: logger}
}

// Enabled reporta si está configurado. Sin client id/secret todo el
// módulo se desactiva con error claro.
func (s *Service) Enabled() bool {
	return s.cfg.ClientID != "" && s.cfg.ClientSecret != "" && s.cfg.RedirectURL != ""
}

// AuthURL del flow inicial — el handler HTTP solo añade state.
func (s *Service) AuthURL(state string) string { return s.cfg.AuthURL(state) }

// CompleteAuth procesa el code recibido en el callback: intercambia por
// tokens, obtiene email del usuario, y persiste la integración en store.
func (s *Service) CompleteAuth(ctx context.Context, tenantID, botID, code string) (store.BotCalendarIntegration, error) {
	if !s.Enabled() {
		return store.BotCalendarIntegration{}, errors.New("calendar service not configured")
	}
	tok, err := s.cfg.ExchangeCode(ctx, code)
	if err != nil {
		return store.BotCalendarIntegration{}, fmt.Errorf("exchange code: %w", err)
	}
	if tok.RefreshToken == "" {
		// Sin refresh token no podemos renovar más adelante — fallo crítico.
		// Si el usuario ya tenía consentimiento previo Google no lo devuelve;
		// access_type=offline + prompt=consent debería forzarlo.
		return store.BotCalendarIntegration{}, errors.New("google did not return refresh_token (need prompt=consent)")
	}
	info, err := s.cfg.FetchUserInfo(ctx, tok.AccessToken)
	if err != nil {
		s.logger.Warn("calendar userinfo failed, using empty email", "error", err)
		// No bloqueante — guardamos sin email.
	}
	expires := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
	saved, err := s.store.UpsertBotCalendarIntegration(ctx, store.BotCalendarIntegration{
		TenantID:             tenantID,
		BotID:                botID,
		Provider:             "google",
		AccountEmail:         info.Email,
		CalendarID:           "primary",
		RefreshTokenPlain:    tok.RefreshToken,
		AccessTokenPlain:     tok.AccessToken,
		AccessTokenExpiresAt: &expires,
		Scopes:               tok.Scope,
	})
	return saved, err
}

// validAccessToken devuelve un access token válido para el bot,
// refrescándolo si está expirado o cerca de expirar (margen 60s). El
// caller no necesita preocuparse de la renovación.
func (s *Service) validAccessToken(ctx context.Context, integ store.BotCalendarIntegration) (string, error) {
	if integ.AccessTokenPlain != "" && integ.AccessTokenExpiresAt != nil &&
		time.Until(*integ.AccessTokenExpiresAt) > 60*time.Second {
		return integ.AccessTokenPlain, nil
	}
	// Toca refresh.
	tok, err := s.cfg.Refresh(ctx, integ.RefreshTokenPlain)
	if err != nil {
		return "", fmt.Errorf("refresh access token: %w", err)
	}
	expires := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
	if err := s.store.UpdateBotCalendarAccessToken(ctx, integ.ID, tok.AccessToken, expires); err != nil {
		// No es fatal — podemos usar el access nuevo aunque no persistamos —
		// pero registramos para investigar.
		s.logger.Warn("persist refreshed access token", "integration", integ.ID, "error", err)
	}
	return tok.AccessToken, nil
}

// FreeSlot es un hueco libre [Start, End) de duración suficiente para
// una reunión. Lo devolvemos al LLM como respuesta a check_availability.
type FreeSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FindFreeSlots calcula huecos libres invirtiendo los busy de Google
// dentro de la ventana [from, to). minDurationMin filtra slots cortos
// para que el LLM solo proponga ranuras donde quepa la visita.
//
// Algoritmo: ordenar busy por start → caminar [from, to) cubriendo huecos
// → cada hueco >= minDurationMin se incluye.
func (s *Service) FindFreeSlots(ctx context.Context, tenantID, botID string, from, to time.Time, minDurationMin int) ([]FreeSlot, error) {
	integ, err := s.store.GetBotCalendarIntegration(ctx, tenantID, botID, "google")
	if err != nil {
		return nil, err
	}
	accessToken, err := s.validAccessToken(ctx, integ)
	if err != nil {
		return nil, err
	}
	busy, err := FreeBusy(ctx, accessToken, integ.CalendarID, from, to)
	if err != nil {
		return nil, err
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })

	free := []FreeSlot{}
	cursor := from
	minD := time.Duration(minDurationMin) * time.Minute
	if minD <= 0 {
		minD = 30 * time.Minute
	}
	for _, b := range busy {
		// Recorta busy al rango pedido.
		if b.End.Before(cursor) {
			continue
		}
		if b.Start.After(cursor) {
			gap := b.Start.Sub(cursor)
			if gap >= minD {
				free = append(free, FreeSlot{Start: cursor, End: b.Start})
			}
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
	}
	// Hueco final desde el último busy hasta to.
	if cursor.Before(to) && to.Sub(cursor) >= minD {
		free = append(free, FreeSlot{Start: cursor, End: to})
	}
	return free, nil
}

// ScheduleMeetingInput es la entrada que la tool calendar_schedule_meeting
// pasa al servicio. Lo separamos del calendar.CreateEventInput para no
// exponer el CalendarID al LLM (lo resolvemos desde la integración).
type ScheduleMeetingInput struct {
	Start         time.Time
	DurationMin   int
	Title         string
	Description   string
	AttendeeEmail string
	TimeZone      string
}

// Schedule reserva un evento en el calendar del bot. Si el lead nos
// dejó email durante la llamada, le llega invitación de Google
// automáticamente — sendUpdates=all.
//
// Devuelve también el calendarID resuelto desde la integración para
// que el caller pueda persistirlo junto al event_id (necesario para
// luego identificar el evento al cancel/reschedule).
func (s *Service) Schedule(ctx context.Context, tenantID, botID string, in ScheduleMeetingInput) (Event, string, error) {
	integ, err := s.store.GetBotCalendarIntegration(ctx, tenantID, botID, "google")
	if err != nil {
		return Event{}, "", err
	}
	accessToken, err := s.validAccessToken(ctx, integ)
	if err != nil {
		return Event{}, integ.CalendarID, err
	}
	if in.DurationMin <= 0 {
		in.DurationMin = 30
	}
	end := in.Start.Add(time.Duration(in.DurationMin) * time.Minute)
	ev, err := CreateEvent(ctx, accessToken, CreateEventInput{
		CalendarID:    integ.CalendarID,
		Summary:       in.Title,
		Description:   in.Description,
		Start:         in.Start,
		End:           end,
		AttendeeEmail: in.AttendeeEmail,
		TimeZone:      in.TimeZone,
	})
	return ev, integ.CalendarID, err
}

// Cancel borra un evento del calendar. eventID y calendarID son los
// que persistimos en scheduled_meetings al crear — el caller los pasa
// después de validar ownership.
//
// Idempotente — un 410 de Google (ya borrado) cuenta como éxito.
func (s *Service) Cancel(ctx context.Context, tenantID, botID, calendarID, eventID string) error {
	integ, err := s.store.GetBotCalendarIntegration(ctx, tenantID, botID, "google")
	if err != nil {
		return err
	}
	accessToken, err := s.validAccessToken(ctx, integ)
	if err != nil {
		return err
	}
	if calendarID == "" {
		calendarID = integ.CalendarID
	}
	return DeleteEvent(ctx, accessToken, calendarID, eventID)
}

// Reschedule mueve un evento existente. Mantiene attendees, título,
// descripción — solo cambia start/end. duration se calcula desde
// in.DurationMin o del start/end original si es 0.
type RescheduleInput struct {
	NewStart    time.Time
	DurationMin int
	TimeZone    string
}

func (s *Service) Reschedule(ctx context.Context, tenantID, botID, calendarID, eventID string, in RescheduleInput) (Event, error) {
	integ, err := s.store.GetBotCalendarIntegration(ctx, tenantID, botID, "google")
	if err != nil {
		return Event{}, err
	}
	accessToken, err := s.validAccessToken(ctx, integ)
	if err != nil {
		return Event{}, err
	}
	if calendarID == "" {
		calendarID = integ.CalendarID
	}
	if in.DurationMin <= 0 {
		in.DurationMin = 30
	}
	end := in.NewStart.Add(time.Duration(in.DurationMin) * time.Minute)
	return PatchEvent(ctx, accessToken, calendarID, eventID, in.NewStart, end, in.TimeZone)
}
