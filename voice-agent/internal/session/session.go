package session

import (
	"context"
	"sync"
	"time"
)

// Event is the structured message exchanged with the WebSocket client.
// Binary audio frames are sent separately on the same socket.
type Event struct {
	Type    string `json:"type"`
	Role    string `json:"role,omitempty"`
	Text    string `json:"text,omitempty"`
	Final   bool   `json:"final,omitempty"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Config is the per-session bot configuration handed in at session creation time.
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

	// Credentials are per-tenant overrides for the provider keys/models. Empty fields fall back
	// to the voice-agent's env defaults.
	Credentials Credentials `json:"credentials,omitempty"`

	// Tools (function calling) que el LLM del provider puede invocar.
	// Los pasamos verbatim al negociar Settings (Deepgram) o
	// session.update.tools (OpenAI). Cuando el provider emita una
	// invocación, el provider implementation llama a InvokeTool del
	// session para que se ejecute via backend.
	Tools []Tool `json:"tools,omitempty"`
}

// Tool es una function exponible al LLM. parameters es JSON Schema.
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
}

// Session is the in-memory representation of a live voice conversation. The provider's Run loop
// reads from AudioIn, pushes audio to AudioOut, and emits Events. The WebSocket handler bridges
// these channels with the network connection.
type Session struct {
	ID        string
	Config    Config
	CreatedAt time.Time

	AudioIn  chan []byte
	AudioOut chan []byte
	Events   chan Event

	cancel       context.CancelFunc
	ctx          context.Context
	mu           sync.Mutex
	closed       bool
	onClose      func()
	onTranscript func(sessionID, role, text string)
	onToolInvoke func(ctx context.Context, sessionID, toolName string, args map[string]any) (content string, ok bool)

	// Last transcripts, for inspection via HTTP API.
	transcript []TranscriptLine
}

type TranscriptLine struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

func New(id string, cfg Config) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		ID:        id,
		Config:    cfg,
		CreatedAt: time.Now().UTC(),
		AudioIn:   make(chan []byte, 64),
		AudioOut: make(chan []byte, 64),
		Events:   make(chan Event, 64),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *Session) Context() context.Context { return s.ctx }

// Close terminates the session. Safe to call multiple times.
func (s *Session) Close(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	// Emit a terminal event for whoever is listening.
	select {
	case s.Events <- Event{Type: "end", Reason: reason}:
	default:
	}
	s.cancel()
	if s.onClose != nil {
		s.onClose()
	}
	close(s.AudioIn)
	close(s.AudioOut)
	close(s.Events)
}

func (s *Session) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Session) SetOnClose(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onClose = fn
}

func (s *Session) AppendTranscript(role, text string) {
	s.mu.Lock()
	s.transcript = append(s.transcript, TranscriptLine{Role: role, Text: text, At: time.Now().UTC()})
	if len(s.transcript) > 500 {
		s.transcript = s.transcript[len(s.transcript)-500:]
	}
	hook := s.onTranscript
	s.mu.Unlock()
	if hook != nil {
		// Fire-and-forget. The hook implementation must be non-blocking or return quickly.
		go hook(s.ID, role, text)
	}
}

// SetOnTranscript installs a callback fired after every persisted transcript line. Used by the
// HTTP API to wire transcripts to the backend webhook.
func (s *Session) SetOnTranscript(fn func(sessionID, role, text string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onTranscript = fn
}

// SetOnToolInvoke installs a callback used by providers to dispatch function
// calls coming from the LLM. fn devuelve el "content" textual que el provider
// reenviará al LLM y un bool indicando si la invocación se ejecutó OK. Si
// el handler está sin instalar (no hay backend), devolver ok=false para que
// el provider responda al LLM con un fallback genérico.
func (s *Session) SetOnToolInvoke(fn func(ctx context.Context, sessionID, toolName string, args map[string]any) (string, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onToolInvoke = fn
}

// InvokeTool dispatcha al callback registrado o devuelve un fallback. Se
// llama desde providers cuando reciben un function_call event.
func (s *Session) InvokeTool(ctx context.Context, toolName string, args map[string]any) (string, bool) {
	s.mu.Lock()
	hook := s.onToolInvoke
	s.mu.Unlock()
	if hook == nil {
		return "Action unavailable.", false
	}
	return hook(ctx, s.ID, toolName, args)
}

func (s *Session) Snapshot() ([]TranscriptLine, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]TranscriptLine, len(s.transcript))
	copy(cp, s.transcript)
	return cp, s.CreatedAt, s.closed
}

// Registry holds active sessions keyed by ID, with TTL-based cleanup.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewRegistry(ttl time.Duration) *Registry {
	r := &Registry{sessions: map[string]*Session{}, ttl: ttl}
	go r.gc()
	return r
}

func (r *Registry) Add(s *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = s
}

func (r *Registry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[id]; ok {
		s.Close("removed")
		delete(r.sessions, id)
	}
}

func (r *Registry) List() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}

func (r *Registry) gc() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for range tick.C {
		cutoff := time.Now().Add(-r.ttl)
		r.mu.Lock()
		for id, s := range r.sessions {
			if s.CreatedAt.Before(cutoff) || s.Closed() {
				s.Close("ttl_expired")
				delete(r.sessions, id)
			}
		}
		r.mu.Unlock()
	}
}
