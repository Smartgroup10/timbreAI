package api

// Google Calendar OAuth flow + connection status + disconnect.
//
// Diseño del state:
//   - state = base64(JSON{botId, tenantId, exp}) firmado con HMAC-SHA256
//     usando JWTSecret (mismo secret que ya guardamos en config).
//   - exp = now+10min para que un state robado caduque rápido.
//   - El callback valida HMAC, comprueba exp, y solo entonces acepta.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"timbre/backend/internal/auth"
	"timbre/backend/internal/store"
)

type calendarStatusResponse struct {
	Connected    bool   `json:"connected"`
	Provider     string `json:"provider,omitempty"`
	AccountEmail string `json:"accountEmail,omitempty"`
	ConnectedAt  string `json:"connectedAt,omitempty"`
}

func (s *Server) handleCalendarStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	integ, err := s.store.GetBotCalendarIntegration(r.Context(), tenantID, botID, "google")
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, calendarStatusResponse{Connected: false})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, calendarStatusResponse{
		Connected:    true,
		Provider:     integ.Provider,
		AccountEmail: integ.AccountEmail,
		ConnectedAt:  integ.ConnectedAt.Format(time.RFC3339),
	})
}

// handleCalendarAuthorize devuelve la URL a la que el frontend debe
// redirigir el navegador para empezar el flow. NO redirigimos aquí
// directamente — el frontend abre una popup o navega manualmente.
func (s *Server) handleCalendarAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.calendar == nil || !s.calendar.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "calendar_not_configured")
		return
	}
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	// Verifica que el bot existe y pertenece al tenant — defensa antes
	// del state.
	if _, err := s.store.GetBot(r.Context(), tenantID, botID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bot_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	state, err := signCalendarState(s.cfg.JWTSecret, calendarState{
		BotID:    botID,
		TenantID: tenantID,
		Exp:      time.Now().Add(10 * time.Minute).Unix(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "state_sign_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"authUrl": s.calendar.AuthURL(state),
	})
}

// handleCalendarCallback recibe el redirect de Google con ?code y ?state.
// Valida el state, intercambia el code por tokens y persiste la
// integración. Termina con un HTML mínimo que dice "puedes cerrar esta
// pestaña" — la UI principal detectará el cambio vía polling/realtime.
//
// Esto NO está bajo requireAuth: Google nos llama sin nuestro JWT en
// la petición. El state firmado es lo que autoriza.
func (s *Server) handleCalendarCallback(w http.ResponseWriter, r *http.Request) {
	if s.calendar == nil || !s.calendar.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "calendar_not_configured")
		return
	}
	code := r.URL.Query().Get("code")
	stateRaw := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	if errParam != "" {
		writeCallbackHTML(w, false, "Authorization denied: "+errParam)
		return
	}
	if code == "" || stateRaw == "" {
		writeCallbackHTML(w, false, "Missing code or state.")
		return
	}
	st, err := verifyCalendarState(s.cfg.JWTSecret, stateRaw)
	if err != nil {
		writeCallbackHTML(w, false, "Invalid or expired state.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if _, err := s.calendar.CompleteAuth(ctx, st.TenantID, st.BotID, code); err != nil {
		s.logger.Warn("calendar CompleteAuth", "tenant", st.TenantID, "bot", st.BotID, "error", err)
		writeCallbackHTML(w, false, "Token exchange failed: "+err.Error())
		return
	}
	s.audit(r, "calendar.connect", "bot", st.BotID, map[string]any{"provider": "google"})
	s.emitRealtime(st.TenantID, "bot.calendar_connected", map[string]any{"botId": st.BotID})
	writeCallbackHTML(w, true, "Calendar connected. You can close this tab.")
}

func (s *Server) handleCalendarDisconnect(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	botID := r.PathValue("id")
	if err := s.store.DeleteBotCalendarIntegration(r.Context(), tenantID, botID, "google"); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "calendar.disconnect", "bot", botID, map[string]any{"provider": "google"})
	s.emitRealtime(tenantID, "bot.calendar_disconnected", map[string]any{"botId": botID})
	w.WriteHeader(http.StatusNoContent)
}

// ─── State firma/verify ─────────────────────────────────────────────────

type calendarState struct {
	BotID    string `json:"b"`
	TenantID string `json:"t"`
	Exp      int64  `json:"e"`
}

func signCalendarState(secret string, st calendarState) (string, error) {
	payload, _ := json.Marshal(st)
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(b64))
	sig := hex.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig, nil
}

func verifyCalendarState(secret, token string) (calendarState, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return calendarState{}, errors.New("state format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return calendarState{}, errors.New("state signature mismatch")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return calendarState{}, err
	}
	var out calendarState
	if err := json.Unmarshal(raw, &out); err != nil {
		return calendarState{}, err
	}
	if time.Now().Unix() > out.Exp {
		return calendarState{}, errors.New("state expired")
	}
	return out, nil
}

// writeCallbackHTML responde con una página minimal — el usuario está
// en una pestaña que abrió desde nuestra UI, así que un mensaje claro
// y un botón "cerrar" es todo lo que necesita.
func writeCallbackHTML(w http.ResponseWriter, ok bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	color := "#1f9d55"
	icon := "✓"
	if !ok {
		color = "#b94326"
		icon = "✗"
	}
	fmt.Fprintf(w, `<!doctype html>
<html lang="es"><head><meta charset="utf-8"><title>timbre.ai</title>
<style>
  body{font:14px/1.5 -apple-system,sans-serif;background:#0f1217;color:#f5f3ee;
       display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
  .card{max-width:420px;padding:32px;text-align:center;background:#1a1d24;border-radius:12px;
        border:1px solid rgba(245,243,238,0.08)}
  .icon{font-size:48px;color:%s;margin-bottom:12px}
  button{margin-top:18px;padding:10px 18px;border:0;border-radius:8px;
         background:#e85d3c;color:white;cursor:pointer;font-size:13px}
</style>
</head><body>
<div class="card">
  <div class="icon">%s</div>
  <p>%s</p>
  <button onclick="window.close()">Cerrar pestaña</button>
</div>
<script>
  // Si fue abierto desde la app principal con window.opener, notificamos para que refresque.
  if (window.opener && !window.opener.closed) {
    try { window.opener.postMessage({type:"timbre.calendar.connected", ok:%t}, "*"); } catch(e){}
  }
</script>
</body></html>`, color, icon, message, ok)
}

// resolverHelper para handlers de calendar: localiza al bot pero también
// verifica que su tenant matchea con el del caller. Reutilizado por
// authorize/disconnect/status. (Usa auth.FromContext implícito en
// tenantScope ya.)
var _ = auth.FromContext // keep import; tests reuse
