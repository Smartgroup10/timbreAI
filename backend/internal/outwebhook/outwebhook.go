// Package outwebhook dispatcha eventos del sistema (call.completed,
// lead.status_changed, etc.) a las URLs configuradas por el tenant.
//
// Diseño:
//   - Dispatcher singleton con un canal buffered. Los productores
//     (handlers) llaman a Dispatch(ctx, event) y vuelven al instante.
//     Un pool de workers vacía el canal y hace los POST.
//   - Firma HMAC-SHA256 con el secret del endpoint, en cabecera
//     X-Timbre-Signature. El receptor verifica el HMAC con su copia
//     del secret para confirmar que el webhook viene de nosotros y
//     no de un atacante que conozca la URL.
//   - Reintentos: 1 reintento a los 30s si la respuesta no fue 2xx.
//     Más allá de eso, queda en webhook_deliveries con error y el
//     operador lo ve en la UI.
//   - Si el endpoint no está suscrito al event_type, lo saltamos
//     antes de hacer cualquier I/O.
package outwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"timbre/backend/internal/store"
)

// Event constants — usadas tanto por los productores (api handlers) como
// por el frontend para validación. Si añades una nueva, añade traducción
// y el case correspondiente donde la disparas.
const (
	EventCallCompleted     = "call.completed"
	EventCallQualified     = "call.qualified"
	EventLeadStatusChanged = "lead.status_changed"
	EventToolInvoked       = "tool.invoked"
)

// AllEvents lista los tipos soportados — lo expone /api/webhook-events
// para que la UI pinte un multi-select.
var AllEvents = []string{
	EventCallCompleted,
	EventCallQualified,
	EventLeadStatusChanged,
	EventToolInvoked,
}

// Event es lo que un productor manda a Dispatch. Payload se serializa a JSON.
type Event struct {
	TenantID string
	Type     string
	Payload  map[string]any
}

// Dispatcher es un singleton. Llamar Dispatch desde cualquier handler.
type Dispatcher struct {
	store  *store.Store
	logger *slog.Logger
	ch     chan Event
	http   *http.Client
}

// New arranca el dispatcher con `workers` goroutines vaciando el canal.
// queueSize es el buffer del canal; si se llena, Dispatch dropea con un
// warn — preferimos perder un evento esporádico que bloquear el handler
// HTTP que originó la mutación.
func New(st *store.Store, logger *slog.Logger, workers, queueSize int) *Dispatcher {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 1024
	}
	d := &Dispatcher{
		store:  st,
		logger: logger,
		ch:     make(chan Event, queueSize),
		http:   &http.Client{Timeout: 8 * time.Second},
	}
	for i := 0; i < workers; i++ {
		go d.worker()
	}
	return d
}

// Dispatch agrega un evento al canal. Non-blocking — si se llena, log y
// drop. Idealmente nunca debería pasar; si pasa, ampliar workers/queue.
func (d *Dispatcher) Dispatch(ev Event) {
	if d == nil {
		return
	}
	if ev.TenantID == "" || ev.Type == "" {
		return
	}
	select {
	case d.ch <- ev:
	default:
		d.logger.Warn("outwebhook queue full, dropping event", "tenant", ev.TenantID, "type", ev.Type)
	}
}

func (d *Dispatcher) worker() {
	for ev := range d.ch {
		d.handle(ev)
	}
}

func (d *Dispatcher) handle(ev Event) {
	// Mismo contexto base con timeout amplio — los workers son background;
	// el req.Do tiene su propio timeout via http.Client.Timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	endpoints, err := d.store.ListWebhookEndpointsForEvent(ctx, ev.TenantID, ev.Type)
	if err != nil {
		d.logger.Error("webhook: list endpoints", "tenant", ev.TenantID, "type", ev.Type, "error", err)
		return
	}
	for _, ep := range endpoints {
		d.deliver(ctx, ep, ev, 1)
	}
}

// deliver hace el POST y persiste el resultado. Si falla y attempt==1,
// agenda un único reintento 30s después en otro worker (via Dispatch
// con una marca interna). Reintentos más allá son ya responsabilidad
// del operador (puede reenviar manualmente desde la UI — TODO).
func (d *Dispatcher) deliver(ctx context.Context, ep store.WebhookEndpoint, ev Event, attempt int) {
	body := map[string]any{
		"event":     ev.Type,
		"tenantId":  ev.TenantID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      ev.Payload,
	}
	payload, _ := json.Marshal(body)
	sig := signHMAC(ep.Secret, payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(payload))
	if err != nil {
		d.logFailedDelivery(ctx, ep, ev, attempt, 0, "build_request: "+err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timbre-Event", ev.Type)
	req.Header.Set("X-Timbre-Tenant", ev.TenantID)
	req.Header.Set("X-Timbre-Signature", "sha256="+sig)
	req.Header.Set("User-Agent", "timbre-webhook/1.0")

	resp, err := d.http.Do(req)
	if err != nil {
		d.logFailedDelivery(ctx, ep, ev, attempt, 0, err.Error())
		d.maybeRetry(ep, ev, attempt)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		d.logFailedDelivery(ctx, ep, ev, attempt, resp.StatusCode, fmt.Sprintf("status_%d", resp.StatusCode))
		d.maybeRetry(ep, ev, attempt)
		return
	}

	// OK. Persistimos la entrega exitosa.
	now := time.Now().UTC()
	_ = d.store.LogWebhookDelivery(ctx, store.WebhookDelivery{
		TenantID:    ev.TenantID,
		EndpointID:  &ep.ID,
		EventType:   ev.Type,
		Payload:     body,
		StatusCode:  resp.StatusCode,
		Attempt:     attempt,
		DeliveredAt: &now,
	})
}

func (d *Dispatcher) logFailedDelivery(ctx context.Context, ep store.WebhookEndpoint, ev Event, attempt, status int, errMsg string) {
	_ = d.store.LogWebhookDelivery(ctx, store.WebhookDelivery{
		TenantID:   ev.TenantID,
		EndpointID: &ep.ID,
		EventType:  ev.Type,
		Payload:    map[string]any{"event": ev.Type, "data": ev.Payload},
		StatusCode: status,
		Error:      errMsg,
		Attempt:    attempt,
	})
	d.logger.Warn("webhook delivery failed",
		"endpoint", ep.ID, "url", ep.URL, "type", ev.Type,
		"attempt", attempt, "status", status, "error", errMsg)
}

func (d *Dispatcher) maybeRetry(ep store.WebhookEndpoint, ev Event, attempt int) {
	if attempt >= 2 {
		return
	}
	go func() {
		// Backoff fijo 30s. Una iteración futura puede usar exponential
		// con jitter; para MVP esto basta.
		time.Sleep(30 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		d.deliver(ctx, ep, ev, attempt+1)
	}()
}

// signHMAC produce hex(HMAC-SHA256(secret, body)). El receptor verifica
// reconstruyendo el HMAC con su copia del secret y comparando en tiempo
// constante. Convención idéntica a Stripe webhooks → familiar.
func signHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
