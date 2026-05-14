package api

// Realtime WebSocket endpoint. El frontend conecta con token JWT en
// query string (los WebSockets no pueden mandar Authorization desde el
// navegador), lo validamos manualmente y suscribimos al hub por tenant.

import (
	"context"
	"net/http"

	"github.com/coder/websocket"

	"timbre/backend/internal/auth"
	"timbre/backend/internal/realtime"
)

func (s *Server) handleRealtime(w http.ResponseWriter, r *http.Request) {
	// Auth manual: el JWT viene en ?token=. Validamos antes del upgrade
	// para no aceptar conexiones anónimas. Las WS no pueden llevar
	// Authorization desde el browser API estándar (subprotocols sería un
	// hack peor que esto).
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "missing_token")
		return
	}
	claims, err := auth.Verify(s.cfg.JWTSecret, tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token")
		return
	}
	// Tenant scope: si es platform_admin con override en ?tenant, se suscribe
	// a ese tenant. Si es user normal, al suyo.
	tenantID := claims.TenantID
	if claims.Role == "platform_admin" {
		if override := r.URL.Query().Get("tenant"); override != "" {
			tenantID = override
		}
	}
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_required")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// CompressionMode: deshabilitado para evitar interacción con buffers.
		CompressionMode: websocket.CompressionDisabled,
		// Origin check: el frontend en desarrollo corre en otro puerto. Si en
		// prod queremos restringir orígenes, configurar OriginPatterns aquí.
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Warn("realtime accept", "error", err)
		return
	}
	// Read limit pequeño — el cliente no nos manda nada significativo.
	conn.SetReadLimit(1024)

	client := realtime.NewClient(tenantID, conn, s.logger.With("component", "realtime", "tenant", tenantID))
	s.realtime.Register(client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Writer en goroutine; reader bloquea aquí para detectar cierre.
	go client.Writer(ctx)
	client.Reader(ctx)

	s.realtime.Unregister(client)
	_ = conn.Close(websocket.StatusNormalClosure, "bye")
}
