// Package calendar habla con Google Calendar API. Lo usamos para:
//   - intercambiar el OAuth auth code por refresh + access tokens
//   - refrescar el access token cuando expira (con el refresh token)
//   - consultar disponibilidad (freeBusy.query)
//   - crear eventos (events.insert) con o sin invitado
//
// Implementación REST directa con net/http — evitamos la dependencia
// google.golang.org/api/calendar/v3 porque tira de protobuf + grpc-go
// pesado y solo necesitamos 3 endpoints. Manual es ~150 líneas y más
// fácil de auditar.
package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

	// Scopes: events para crear/leer eventos del primary calendar.
	// readonly cubre freeBusy y listing — separado por si más adelante
	// queremos pedir solo lectura para algunos clientes.
	GoogleScopeEvents   = "https://www.googleapis.com/auth/calendar.events"
	GoogleScopeReadonly = "https://www.googleapis.com/auth/calendar.readonly"
)

// GoogleScopes para nuestra integración. Pedimos los mínimos.
var GoogleScopes = []string{GoogleScopeEvents, GoogleScopeReadonly}

// OAuthConfig se construye una vez al arrancar el server con las creds
// del Google Cloud Console.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// AuthURL devuelve la URL a la que redirigir al navegador para empezar
// el flow. state es un nonce con el bot_id/tenant_id firmado HMAC para
// validar en el callback (evita CSRF y permite saber qué bot conecta).
//
// access_type=offline + prompt=consent garantiza que Google entregue
// un refresh_token (sin estos, en re-conexiones solo da access_token).
func (c OAuthConfig) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(GoogleScopes, " "))
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("state", state)
	return googleAuthURL + "?" + q.Encode()
}

// TokenResponse es lo que devuelve Google al cambiar code/refresh por tokens.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"` // solo en el primer intercambio
	ExpiresIn    int    `json:"expires_in"`              // segundos
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// ExchangeCode cambia el auth code recibido en el callback por
// access + refresh tokens. Solo se llama una vez por conexión.
func (c OAuthConfig) ExchangeCode(ctx context.Context, code string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.RedirectURL)
	return c.postToken(ctx, form)
}

// Refresh usa el refresh token (long-lived) para obtener un access
// token nuevo cuando el actual está cerca de expirar. Google NO
// devuelve nuevo refresh_token aquí — el de la integración sigue válido.
func (c OAuthConfig) Refresh(ctx context.Context, refreshToken string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")
	return c.postToken(ctx, form)
}

func (c OAuthConfig) postToken(ctx context.Context, form url.Values) (TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return TokenResponse{}, fmt.Errorf("google token %d: %s", resp.StatusCode, string(body))
	}
	var out TokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return TokenResponse{}, err
	}
	return out, nil
}

// UserInfo es el subset que nos interesa para mostrar "conectado como
// X" en la UI. Lo pedimos justo después del exchange para identificar
// la cuenta.
type UserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (c OAuthConfig) FetchUserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", googleUserInfoURL, nil)
	if err != nil {
		return UserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UserInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return UserInfo{}, fmt.Errorf("google userinfo %d: %s", resp.StatusCode, string(b))
	}
	var out UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return UserInfo{}, err
	}
	return out, nil
}

// ─── Calendar operations ────────────────────────────────────────────────

// FreeBusySlot representa un periodo ocupado devuelto por freeBusy.query.
type FreeBusySlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FreeBusy devuelve los huecos OCUPADOS del calendarId entre [from, to).
// La tool del bot luego invierte esto a "huecos libres" — Google solo
// devuelve los busy, no los free.
func FreeBusy(ctx context.Context, accessToken, calendarID string, from, to time.Time) ([]FreeBusySlot, error) {
	body := map[string]any{
		"timeMin": from.Format(time.RFC3339),
		"timeMax": to.Format(time.RFC3339),
		"items":   []map[string]string{{"id": calendarID}},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.googleapis.com/calendar/v3/freeBusy", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google freeBusy %d: %s", resp.StatusCode, string(rb))
	}
	var out struct {
		Calendars map[string]struct {
			Busy []struct {
				Start time.Time `json:"start"`
				End   time.Time `json:"end"`
			} `json:"busy"`
		} `json:"calendars"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, err
	}
	cal, ok := out.Calendars[calendarID]
	if !ok {
		return nil, nil
	}
	slots := make([]FreeBusySlot, 0, len(cal.Busy))
	for _, b := range cal.Busy {
		slots = append(slots, FreeBusySlot{Start: b.Start, End: b.End})
	}
	return slots, nil
}

// Event es lo mínimo que devolvemos cuando creamos uno — para mostrarlo
// en el detalle de llamada y permitir borrar/editar más adelante.
type Event struct {
	ID      string `json:"id"`
	HTMLink string `json:"htmlLink"`
	Status  string `json:"status"`
}

// CreateEventInput agrupa los parámetros de events.insert. Mantenemos
// el nombre y descripción mínimos para el MVP — más adelante podríamos
// añadir location, conferenceData (Google Meet auto), recurrence, etc.
type CreateEventInput struct {
	CalendarID    string
	Summary       string // título del evento
	Description   string
	Start         time.Time
	End           time.Time
	AttendeeEmail string // opcional; si está, Google manda invitación
	TimeZone      string // p.ej. "Europe/Madrid"; default "UTC"
}

// DeleteEvent cancela un evento. sendUpdates=all hace que Google
// notifique al invitado (el lead) por email. La API devuelve 410 Gone
// si el evento ya estaba eliminado — lo tratamos como éxito porque
// el estado final es el deseado.
func DeleteEvent(ctx context.Context, accessToken, calendarID, eventID string) error {
	urlStr := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s?sendUpdates=all",
		url.PathEscape(calendarID), url.PathEscape(eventID))
	req, err := http.NewRequestWithContext(ctx, "DELETE", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 204 || resp.StatusCode == 200 || resp.StatusCode == 410 {
		// 410 = ya cancelado o borrado; idempotente.
		return nil
	}
	rb, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("google events.delete %d: %s", resp.StatusCode, string(rb))
}

// PatchEvent actualiza un evento. Para reschedule solo necesitamos
// start y end — usamos PATCH (no PUT) para no tener que reenviar
// todos los campos (summary, attendees, etc.).
func PatchEvent(ctx context.Context, accessToken, calendarID, eventID string, newStart, newEnd time.Time, timezone string) (Event, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	body := map[string]any{
		"start": map[string]string{"dateTime": newStart.Format(time.RFC3339), "timeZone": timezone},
		"end":   map[string]string{"dateTime": newEnd.Format(time.RFC3339), "timeZone": timezone},
	}
	raw, _ := json.Marshal(body)
	urlStr := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s?sendUpdates=all",
		url.PathEscape(calendarID), url.PathEscape(eventID))
	req, err := http.NewRequestWithContext(ctx, "PATCH", urlStr, bytes.NewReader(raw))
	if err != nil {
		return Event{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Event{}, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return Event{}, fmt.Errorf("google events.patch %d: %s", resp.StatusCode, string(rb))
	}
	var ev Event
	if err := json.Unmarshal(rb, &ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// CreateEvent inserta un evento. Si AttendeeEmail está, Google envía
// automáticamente invitación email al lead (sendUpdates=all).
func CreateEvent(ctx context.Context, accessToken string, in CreateEventInput) (Event, error) {
	if in.TimeZone == "" {
		in.TimeZone = "UTC"
	}
	body := map[string]any{
		"summary":     in.Summary,
		"description": in.Description,
		"start":       map[string]string{"dateTime": in.Start.Format(time.RFC3339), "timeZone": in.TimeZone},
		"end":         map[string]string{"dateTime": in.End.Format(time.RFC3339), "timeZone": in.TimeZone},
	}
	if in.AttendeeEmail != "" {
		body["attendees"] = []map[string]string{{"email": in.AttendeeEmail}}
	}
	raw, _ := json.Marshal(body)
	urlStr := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?sendUpdates=all",
		url.PathEscape(in.CalendarID))
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(raw))
	if err != nil {
		return Event{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Event{}, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return Event{}, fmt.Errorf("google events.insert %d: %s", resp.StatusCode, string(rb))
	}
	var ev Event
	if err := json.Unmarshal(rb, &ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}
