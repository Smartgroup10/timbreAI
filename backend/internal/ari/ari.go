package ari

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type Client struct {
	baseURL  string
	user     string
	password string
	app      string
	http     *http.Client
	logger   *slog.Logger
}

func New(baseURL, user, password, app string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		user:     user,
		password: password,
		app:      app,
		http:     &http.Client{Timeout: 15 * time.Second},
		logger:   logger,
	}
}

func (c *Client) auth() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.user+":"+c.password))
}

type OriginateRequest struct {
	Endpoint   string            `json:"endpoint"`
	App        string            `json:"app"`
	AppArgs    string            `json:"appArgs,omitempty"`
	CallerID   string            `json:"callerId,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
	ChannelID  string            `json:"channelId,omitempty"`
}

type Channel struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Name  string `json:"name"`
}

// Originate creates a new outbound channel. Asterisk dials `Endpoint` (e.g. PJSIP/6001 or
// PJSIP/+14155551234@trunk) and on answer hands control to the Stasis app.
func (c *Client) Originate(ctx context.Context, req OriginateRequest) (Channel, error) {
	if req.App == "" {
		req.App = c.app
	}
	if req.Timeout == 0 {
		req.Timeout = 30
	}
	body, err := json.Marshal(req)
	if err != nil {
		return Channel{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/channels", bytes.NewReader(body))
	if err != nil {
		return Channel{}, err
	}
	httpReq.Header.Set("Authorization", c.auth())
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Channel{}, fmt.Errorf("ari originate: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return Channel{}, fmt.Errorf("ari originate: %s: %s", resp.Status, string(respBody))
	}
	var ch Channel
	if err := json.Unmarshal(respBody, &ch); err != nil {
		return Channel{}, fmt.Errorf("ari decode: %w", err)
	}
	return ch, nil
}

// CreateExternalMediaChannel asks Asterisk to open a UDP socket and stream audio (in both
// directions) to ExternalHost. Asterisk picks its own source port; the voice-agent learns it
// from the first inbound packet.
type ExternalMediaRequest struct {
	App          string `json:"app"`
	ExternalHost string `json:"external_host"`
	Format       string `json:"format"`
	ChannelID    string `json:"channelId,omitempty"`
}

func (c *Client) CreateExternalMedia(ctx context.Context, req ExternalMediaRequest) (Channel, error) {
	if req.App == "" {
		req.App = c.app
	}
	if req.Format == "" {
		req.Format = "slin16"
	}
	body, err := json.Marshal(req)
	if err != nil {
		return Channel{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/channels/externalMedia", bytes.NewReader(body))
	if err != nil {
		return Channel{}, err
	}
	httpReq.Header.Set("Authorization", c.auth())
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Channel{}, fmt.Errorf("external media: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return Channel{}, fmt.Errorf("external media: %s: %s", resp.Status, string(b))
	}
	var ch Channel
	if err := json.Unmarshal(b, &ch); err != nil {
		return Channel{}, err
	}
	return ch, nil
}

// CreateBridge opens a mixing bridge so we can attach the inbound channel + external media one.
func (c *Client) CreateBridge(ctx context.Context, bridgeType string) (string, error) {
	body, _ := json.Marshal(map[string]string{"type": bridgeType, "name": "callhub-" + bridgeType})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridges", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", c.auth())
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("create bridge: %s: %s", resp.Status, string(b))
	}
	var br struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &br); err != nil {
		return "", err
	}
	return br.ID, nil
}

func (c *Client) AddChannelToBridge(ctx context.Context, bridgeID, channelID string) error {
	u := c.baseURL + "/bridges/" + url.PathEscape(bridgeID) + "/addChannel"
	q := "?channel=" + url.QueryEscape(channelID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u+q, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", c.auth())
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add channel: %s: %s", resp.Status, string(b))
	}
	return nil
}

// AnswerChannel tells Asterisk to send the 200 OK so the caller's audio starts flowing.
func (c *Client) AnswerChannel(ctx context.Context, channelID string) error {
	u := c.baseURL + "/channels/" + url.PathEscape(channelID) + "/answer"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", c.auth())
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("answer channel: %s: %s", resp.Status, string(b))
	}
	return nil
}

// Endpoint represents a PJSIP endpoint as Asterisk sees it. State is "online",
// "offline" o "unknown" — para un trunk con REGISTER, "online" implica registro
// activo; para un trunk con Identify-by-IP, "online" requiere qualify_frequency
// configurado y respuesta a OPTIONS.
type Endpoint struct {
	Technology string   `json:"technology"`
	Resource   string   `json:"resource"`
	State      string   `json:"state"`
	ChannelIDs []string `json:"channel_ids"`
}

// ReloadModule pide a Asterisk que recargue un módulo concreto (equivalente a
// "module reload <name>" en la CLI). Lo usamos tras crear/editar un trunk en
// realtime para forzar a Asterisk a re-leer ps_registrations (las registrations
// salientes se cachean al startup; los endpoints/auths/aors son lazy-loaded y
// no necesitan reload).
func (c *Client) ReloadModule(ctx context.Context, name string) error {
	u := c.baseURL + "/asterisk/modules/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth())
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ari reload %s: %s: %s", name, resp.Status, string(body))
	}
	return nil
}

// ListEndpoints devuelve los endpoints registrados en Asterisk para una tecnología
// (típicamente "PJSIP"). Usado por la UI admin para mostrar el estado real del
// registro contra el proveedor SIP.
func (c *Client) ListEndpoints(ctx context.Context, tech string) ([]Endpoint, error) {
	u := c.baseURL + "/endpoints"
	if tech != "" {
		u = u + "/" + url.PathEscape(tech)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ari list endpoints: %s: %s", resp.Status, string(body))
	}
	var out []Endpoint
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) HangupChannel(ctx context.Context, channelID string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/channels/"+url.PathEscape(channelID), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", c.auth())
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ari hangup: %s: %s", resp.Status, string(body))
	}
	return nil
}

// Event is a minimally parsed ARI websocket event. We only care about a handful of fields here;
// downstream consumers can re-parse the raw payload if they need more.
type Event struct {
	Type      string          `json:"type"`
	Channel   *Channel        `json:"channel,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// EventHandler receives every ARI event delivered over the websocket. Implementations should be
// non-blocking; long work should be dispatched to goroutines or a queue.
type EventHandler func(ctx context.Context, ev Event)

// RunEventLoop opens the ARI websocket (subscribing to our Stasis app) and dispatches events to
// the handler. It blocks until ctx is cancelled or the websocket terminates unrecoverably.
func (c *Client) RunEventLoop(ctx context.Context, handler EventHandler) error {
	wsURL, err := buildEventsURL(c.baseURL, c.app, c.user, c.password)
	if err != nil {
		return err
	}

	backoff := time.Second
	for {
		if err := c.runOnce(ctx, wsURL, handler); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Warn("ari ws disconnected", "error", err, "retry_in", backoff)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (c *Client) runOnce(ctx context.Context, wsURL string, handler EventHandler) error {
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "shutting down")
	c.logger.Info("ari ws connected", "app", c.app)

	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var ev Event
		if err := json.Unmarshal(msg, &ev); err != nil {
			c.logger.Warn("ari ws decode", "error", err)
			continue
		}
		ev.Raw = append(json.RawMessage(nil), msg...)
		handler(ctx, ev)
	}
}

func buildEventsURL(baseURL, app, user, password string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/events"
	q := u.Query()
	q.Set("app", app)
	q.Set("api_key", user+":"+password)
	q.Set("subscribeAll", "true")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ErrDisabled is returned by helpers when ARI is intentionally disabled at config level.
var ErrDisabled = errors.New("ari_disabled")
