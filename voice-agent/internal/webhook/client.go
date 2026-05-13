package webhook

import (
	"bytes"
	"context"
	"encoding/json"
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
