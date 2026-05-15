package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Client posts transcript events from the voice-agent to the backend. Failures are logged but do
// not interrupt the live conversation — transcripts are a "nice-to-have" side effect of the call.
type Client struct {
	url    string
	secret string
	http   *http.Client
	logger *slog.Logger
}

func New(baseURL, secret string, logger *slog.Logger) *Client {
	return &Client{
		url:    strings.TrimRight(baseURL, "/"),
		secret: secret,
		http:   &http.Client{Timeout: 3 * time.Second},
		logger: logger,
	}
}

func (c *Client) Enabled() bool { return c.url != "" }

type TranscriptInput struct {
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Text      string `json:"text"`
}

func (c *Client) PostTranscript(ctx context.Context, in TranscriptInput) {
	if !c.Enabled() {
		return
	}
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/internal/voice/transcripts", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Internal-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("transcript webhook", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != 404 {
		c.logger.Warn("transcript webhook non-2xx", "status", resp.StatusCode)
	}
}

// AMDResultInput informa al backend del veredicto del detector de buzón.
// El backend persiste calls.amd_result y decide si cerrar la sesión
// (action=hangup) o solo marcarla (action=drop_message/continue).
type AMDResultInput struct {
	SessionID string `json:"sessionId"`
	BotID     string `json:"botId,omitempty"`
	Result    string `json:"result"` // "human" | "machine" | "unknown"
}

// UsageInput informa al backend del consumo de la sesión al cerrar.
// Los contadores por componente (tokens, chars) se rellenan cuando
// instrumentemos cada provider; por ahora reportamos DurationSec que
// el backend ya usa de fallback con tarifa flat.
type UsageInput struct {
	SessionID       string `json:"sessionId"`
	DurationSec     int    `json:"durationSec"`
	STTSeconds      int    `json:"sttSeconds,omitempty"`
	LLMInputTokens  int    `json:"llmInputTokens,omitempty"`
	LLMOutputTokens int    `json:"llmOutputTokens,omitempty"`
	TTSChars        int    `json:"ttsChars,omitempty"`
	TTSSeconds      int    `json:"ttsSeconds,omitempty"`
}

// PostUsage reporta los contadores de consumo al backend al cerrar la
// sesión. Fire-and-forget — fallar aquí no rompe la llamada ya cerrada.
func (c *Client) PostUsage(ctx context.Context, in UsageInput) {
	if !c.Enabled() {
		return
	}
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/internal/voice/usage", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Internal-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("usage webhook", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != 404 {
		c.logger.Warn("usage webhook non-2xx", "status", resp.StatusCode)
	}
}

// PostAMDResult notifica al backend el veredicto del AMD detector.
// Fire-and-forget — si falla no rompe la llamada, solo perdemos el flag.
func (c *Client) PostAMDResult(ctx context.Context, in AMDResultInput) {
	if !c.Enabled() {
		return
	}
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/internal/voice/amd-result", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Internal-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("amd webhook", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != 404 {
		c.logger.Warn("amd webhook non-2xx", "status", resp.StatusCode)
	}
}

// ToolInvokeInput es la petición que enviamos al backend cuando un provider
// emite un function_call durante la llamada.
type ToolInvokeInput struct {
	SessionID string         `json:"sessionId"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
}

// ToolInvokeResult es lo que devuelve el backend. Content es lo que enviamos
// al provider como contenido del FunctionCallResponse.
type ToolInvokeResult struct {
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// InvokeTool llama al backend para ejecutar una tool. Síncrono — el provider
// está esperando la respuesta. Timeout corto para no atascar la sesión: si
// el backend tarda, devolvemos un fallback genérico.
func (c *Client) InvokeTool(ctx context.Context, in ToolInvokeInput) (ToolInvokeResult, error) {
	if !c.Enabled() {
		return ToolInvokeResult{Success: false, Content: "Action unavailable.", Error: "no_backend"}, nil
	}
	body, _ := json.Marshal(in)
	// Subimos el timeout porque el backend puede tener que hablar con CRMs
	// externos. Aún así limitado para no bloquear la conversación.
	hctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodPost, c.url+"/api/internal/voice/tool-invoke", bytes.NewReader(body))
	if err != nil {
		return ToolInvokeResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Internal-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("tool invoke webhook", "error", err)
		return ToolInvokeResult{Success: false, Content: "Action timed out.", Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return ToolInvokeResult{Success: false, Content: "Action failed.", Error: fmt.Sprintf("status_%d", resp.StatusCode)},
			fmt.Errorf("tool invoke %d: %s", resp.StatusCode, string(b))
	}
	var out ToolInvokeResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ToolInvokeResult{}, err
	}
	return out, nil
}
