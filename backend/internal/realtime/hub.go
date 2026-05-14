// Package realtime push-deliver eventos del backend al frontend vía
// WebSocket — reemplaza el polling de listas (calls/leads/dnc/campaigns).
//
// Topología:
//   - Hub con un map tenantID → set de clients suscritos.
//   - Cada cliente es una goroutine que copia mensajes desde su canal
//     send al WebSocket. Si el canal se llena (cliente lento), lo
//     cerramos y dejamos que se reconecte — preferimos perder un cliente
//     que bloquear el broadcast del tenant entero.
//   - Eventos van por tenant_id. Un platform_admin que impersone un
//     tenant recibe los eventos de ESE tenant (el frontend pasa el
//     ?tenant en el query string al upgrade).
//
// El protocolo es JSON minimal: {"type": "...", "data": {...}}. No hace
// falta nada más sofisticado — el frontend reacciona haciendo refetch
// (no diffing fino), así que el evento solo necesita decir "algo cambió
// en este recurso".
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Event constants para evitar typos entre productor y consumidor.
const (
	EventCallCreated  = "call.created"
	EventCallUpdated  = "call.updated"
	EventCallFinished = "call.finished"
	EventLeadCreated  = "lead.created"
	EventLeadUpdated  = "lead.updated"
	EventLeadDeleted  = "lead.deleted"
	EventCampaign     = "campaign.updated"
	EventDNCChanged   = "dnc.changed"
	EventToolInvoked  = "tool.invoked"
)

// Event es lo que se serializa al cliente.
type Event struct {
	Type     string         `json:"type"`
	TenantID string         `json:"tenantId"`
	Data     map[string]any `json:"data,omitempty"`
}

// Hub es singleton, instanciado al arrancar el server.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
	logger  *slog.Logger
}

// Client es una conexión WebSocket suscrita a un tenant. send tiene
// buffer pequeño — si se llena, el cliente se desconecta.
type Client struct {
	tenantID string
	conn     *websocket.Conn
	send     chan []byte
	logger   *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: map[string]map[*Client]struct{}{},
		logger:  logger,
	}
}

// Register añade un cliente y arranca su writer goroutine. El handler
// HTTP debe seguir con Wait() (lectura) en su propia goroutine para
// detectar cierres.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.tenantID] == nil {
		h.clients[c.tenantID] = map[*Client]struct{}{}
	}
	h.clients[c.tenantID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[c.tenantID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.tenantID)
		}
	}
	close(c.send)
}

// Broadcast envía el evento a todos los clientes del tenant. Non-blocking
// por cliente — si su send está lleno, lo dropeamos del set para forzar
// reconexión. No serializamos por cliente: una vez basta para todos.
//
// Mantenemos el lock durante los sends para evitar el race "Unregister
// cierra send mientras Broadcast intenta escribir" → panic. Los sends son
// non-blocking (default branch), así que el lock se libera al instante;
// es más barato y simple que un mutex per-client.
func (h *Hub) Broadcast(ev Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		h.logger.Warn("realtime marshal", "error", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.clients[ev.TenantID]
	for c := range set {
		select {
		case c.send <- payload:
		default:
			h.logger.Warn("realtime client send full, dropping", "tenant", c.tenantID)
			_ = c.conn.Close(websocket.StatusPolicyViolation, "send_buffer_full")
		}
	}
}

// NewClient crea un cliente y arranca su writer. El caller debe
// llamar Register en el Hub y luego runReader() para bloquear hasta
// que el cliente cierre.
func NewClient(tenantID string, conn *websocket.Conn, logger *slog.Logger) *Client {
	return &Client{
		tenantID: tenantID,
		conn:     conn,
		send:     make(chan []byte, 32),
		logger:   logger,
	}
}

// Writer bombea desde send al socket. Se llama en una goroutine.
// También envía pings periódicos para mantener viva la conexión a
// través de proxies/balancers que cortan idle TCP.
func (c *Client) Writer(ctx context.Context) {
	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-pingTicker.C:
			pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// Reader es un bucle vacío — el cliente no nos manda nada (por ahora).
// Lo necesitamos para detectar cierres del lado del cliente (Read
// devuelve error al cerrarse el socket) y limpiar el registro.
func (c *Client) Reader(ctx context.Context) {
	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
	}
}
