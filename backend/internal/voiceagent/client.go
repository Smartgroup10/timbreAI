package voiceagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the voice-agent service. Sessions are created when a call originates so the
// RTP bridge (or any test client) can hook in via the audio WebSocket using the returned id.
type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

func New(baseURL, secret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		http:    &http.Client{Timeout: 8 * time.Second},
	}
}

type Config struct {
	CallID     string   `json:"callId"`
	TenantID   string   `json:"tenantId"`
	BotID      string   `json:"botId"`
	Provider   string   `json:"provider"`
	Objective  string   `json:"objective"`
	Guardrails []string `json:"guardrails"`
	Language   string   `json:"language"`
	Voice      string   `json:"voice"`
	LeadName   string   `json:"leadName,omitempty"`

	// Per-tenant overrides for provider credentials. Empty fields fall back to env defaults.
	Credentials Credentials `json:"credentials,omitempty"`

	// Tools (function calling) que el bot puede invocar. El voice-agent
	// las pasa al provider en la negociación inicial; cuando el provider
	// emite un function_call, el voice-agent llama a /api/internal/voice/tool-invoke.
	Tools []Tool `json:"tools,omitempty"`

	// AMD (Answering Machine Detection) — config tomada del bot.
	AMD AMDConfig `json:"amd,omitempty"`
}

// AMDConfig configura el detector de buzón del voice-agent.
type AMDConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Action  string `json:"action,omitempty"`
	Message string `json:"message,omitempty"`
}

// Tool es la definición que se envía al provider de voz para function
// calling. parameters es JSON Schema; los providers (OpenAI, Deepgram)
// aceptan el mismo formato.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Credentials struct {
	OpenAIAPIKey        string `json:"openaiApiKey,omitempty"`
	OpenAIRealtimeModel string `json:"openaiRealtimeModel,omitempty"`
	OpenAIRealtimeVoice string `json:"openaiRealtimeVoice,omitempty"`

	DeepgramAPIKey        string `json:"deepgramApiKey,omitempty"`
	DeepgramListenModel   string `json:"deepgramListenModel,omitempty"`
	DeepgramThinkProvider string `json:"deepgramThinkProvider,omitempty"`
	DeepgramThinkModel    string `json:"deepgramThinkModel,omitempty"`
	DeepgramSpeakModel    string `json:"deepgramSpeakModel,omitempty"`
	DeepgramGreeting      string `json:"deepgramGreeting,omitempty"`

	AssemblyAIAPIKey   string `json:"assemblyaiApiKey,omitempty"`
	AssemblyAIVoice    string `json:"assemblyaiVoice,omitempty"`
	AssemblyAIGreeting string `json:"assemblyaiGreeting,omitempty"`

	ElevenLabsAPIKey  string `json:"elevenlabsApiKey,omitempty"`
	ElevenLabsAgentID string `json:"elevenlabsAgentId,omitempty"`
}

type Session struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	AudioWsPath string `json:"audioWsPath"`
}

func (c *Client) Enabled() bool { return c.baseURL != "" }

// CreateSession registers a new conversation with the voice-agent. Returns the session id which
// the RTP bridge will use to connect on /sessions/{id}/audio.
func (c *Client) CreateSession(ctx context.Context, cfg Config) (Session, error) {
	if !c.Enabled() {
		return Session{}, errors.New("voice_agent_not_configured")
	}
	body, _ := json.Marshal(cfg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sessions", bytes.NewReader(body))
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Voice-Agent-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Session{}, fmt.Errorf("voice-agent create session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return Session{}, fmt.Errorf("voice-agent %d: %s", resp.StatusCode, string(b))
	}
	var s Session
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// Providers asks the voice-agent which providers are currently configured (echo + whichever
// have API keys). Used by the admin Operations page to show the real state.
func (c *Client) Providers(ctx context.Context) ([]string, error) {
	if !c.Enabled() {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/providers", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("voice-agent providers %d: %s", resp.StatusCode, string(b))
	}
	var body struct {
		Providers []string `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Providers, nil
}

type RTPEndpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// AllocateRTP asks the voice-agent to bind a UDP port for the session and start the RTP bridge.
// The returned host:port is what we hand to Asterisk's External Media channel as `external_host`.
func (c *Client) AllocateRTP(ctx context.Context, sessionID string) (RTPEndpoint, error) {
	var out RTPEndpoint
	if !c.Enabled() {
		return out, errors.New("voice_agent_not_configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sessions/"+sessionID+"/rtp", nil)
	if err != nil {
		return out, err
	}
	if c.secret != "" {
		req.Header.Set("X-Voice-Agent-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return out, fmt.Errorf("voice-agent rtp %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) EndSession(ctx context.Context, id string) error {
	if !c.Enabled() {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/sessions/"+id, nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("X-Voice-Agent-Secret", c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
