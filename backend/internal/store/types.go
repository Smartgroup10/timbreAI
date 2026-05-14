package store

import "time"

type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID           string     `json:"id"`
	TenantID     *string    `json:"tenantId,omitempty"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Role         string     `json:"role"`
	PasswordHash string     `json:"-"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type Lead struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenantId"`
	Name         string    `json:"name"`
	Phone        string    `json:"phone"`
	Email        string    `json:"email"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Source       string    `json:"source"`
	Consent      string    `json:"consent"`
	LastActivity time.Time `json:"lastActivity"`
}

type Property struct {
	ID           string   `json:"id"`
	TenantID     string   `json:"tenantId"`
	Name         string   `json:"name"`
	Address      string   `json:"address"`
	Price        string   `json:"price"`
	Availability string   `json:"availability"`
	Requirements []string `json:"requirements"`
	FAQs         []string `json:"faqs"`
}

type Bot struct {
	ID            string   `json:"id"`
	TenantID      string   `json:"tenantId"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Language      string   `json:"language"`
	Voice         string   `json:"voice"`
	Status        string   `json:"status"`
	Objective     string   `json:"objective"`
	Guardrails    []string `json:"guardrails"`
	VoiceProvider string   `json:"voiceProvider"`
	DIDID         *string  `json:"didId,omitempty"`
	DIDE164       string   `json:"didE164,omitempty"`
	TrunkID       string   `json:"trunkId,omitempty"`
}

type SIPTrunk struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Provider         string    `json:"provider"`
	AsteriskEndpoint string    `json:"asteriskEndpoint"`
	Host             string    `json:"host"`
	Port             int       `json:"port"`
	Username         string    `json:"username"`
	Password         string    `json:"password,omitempty"` // solo se devuelve enmascarado al frontend
	Register         bool      `json:"register"`
	IdentifyIP       string    `json:"identifyIp"`
	Status           string    `json:"status"`
	Notes            string    `json:"notes"`
	DIDCount         int       `json:"didCount"`
	CreatedAt        time.Time `json:"createdAt"`
}

type DID struct {
	ID               string    `json:"id"`
	TrunkID          string    `json:"trunkId"`
	TrunkName        string    `json:"trunkName,omitempty"`
	AsteriskEndpoint string    `json:"asteriskEndpoint,omitempty"`
	TenantID         *string   `json:"tenantId,omitempty"`
	E164             string    `json:"e164"`
	Label            string    `json:"label"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Campaign struct {
	ID                   string     `json:"id"`
	TenantID             string     `json:"tenantId"`
	Name                 string     `json:"name"`
	BotID                string     `json:"botId"`
	Status               string     `json:"status"`
	Schedule             string     `json:"schedule"`
	LeadCount            int        `json:"leadCount"`
	MaxAttempts          int        `json:"maxAttempts"`
	RetryCooldownMinutes int        `json:"retryCooldownMinutes"`
	StartAt              *time.Time `json:"startAt,omitempty"`
	EndAt                *time.Time `json:"endAt,omitempty"`
	MaxConcurrent        int        `json:"maxConcurrent"`
}

type CampaignLead struct {
	ID            string  `json:"id"`
	TenantID      string  `json:"tenantId"`
	CampaignID    string  `json:"campaignId"`
	LeadID        string  `json:"leadId"`
	LeadName      string  `json:"leadName,omitempty"`
	LeadPhone     string  `json:"leadPhone,omitempty"`
	Status        string  `json:"status"`
	Attempts      int     `json:"attempts"`
	LastAttemptAt *string `json:"lastAttemptAt,omitempty"`
	Outcome       string  `json:"outcome"`
}

type Call struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenantId"`
	LeadID         *string    `json:"leadId,omitempty"`
	CampaignID     *string    `json:"campaignId,omitempty"`
	LeadName       string     `json:"leadName"`
	Campaign       string     `json:"campaign"`
	Phone          string     `json:"phone"`
	Status         string     `json:"status"`
	Outcome        string     `json:"outcome"`
	DurationSec    int        `json:"durationSec"`
	ChannelID      string     `json:"channelId"`
	VoiceSessionID string     `json:"voiceSessionId,omitempty"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	Summary        string     `json:"summary"`
	RecordingURL   string     `json:"recordingUrl,omitempty"`
	// Provider que atendió la llamada (openai_realtime/deepgram/assemblyai/echo).
	// Snapshot al crear la llamada — el bot podría cambiar de provider más
	// tarde y haríamos un cálculo de coste inconsistente.
	Provider string `json:"provider,omitempty"`
	// CostCents es coste estimado en céntimos. NO se persiste; lo calcula
	// el handler al serializar a partir de provider × duration y la tabla
	// de tarifas activa. Si cambias tarifas las llamadas viejas reflejan
	// el nuevo precio (estimación, no facturación).
	CostCents int `json:"costCents"`
}

type Overview struct {
	CallsToday      int `json:"callsToday"`
	QualifiedLeads  int `json:"qualifiedLeads"`
	Callbacks       int `json:"callbacks"`
	ActiveCampaigns int `json:"activeCampaigns"`
	QueuedCalls     int `json:"queuedCalls"`
}

// BotTool define una "function" que el LLM del bot puede invocar durante
// una llamada (function calling / tool use). Cada provider de voz acepta
// estas funciones en su Settings inicial. Cuando el LLM decide llamarla,
// el voice-agent reenvía la petición al backend, que ejecuta el ActionType
// correspondiente y devuelve el resultado al provider.
type BotTool struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenantId"`
	BotID            string         `json:"botId"`
	Name             string         `json:"name"`        // "set_qualified", "schedule_visit"
	Description      string         `json:"description"` // lo que lee el LLM para decidir cuándo llamar
	ParametersSchema map[string]any `json:"parametersSchema"`
	ActionType       string         `json:"actionType"`
	ActionConfig     map[string]any `json:"actionConfig"`
	Enabled          bool           `json:"enabled"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// BotToolInvocation registra cada vez que el LLM invocó una tool durante
// una llamada — útil para auditar y debuggear comportamiento del bot.
type BotToolInvocation struct {
	ID        string         `json:"id"`
	TenantID  string         `json:"tenantId"`
	CallID    *string        `json:"callId,omitempty"`
	BotToolID *string        `json:"botToolId,omitempty"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
	Result    map[string]any `json:"result"`
	Success   bool           `json:"success"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// WebhookEndpoint es la suscripción de un tenant a un canal CRM. Se
// dispara cuando suceden eventos del tipo listado en Events.
type WebhookEndpoint struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	// Secret se muestra UNA VEZ al crear (response JSON) y luego nunca
	// se devuelve en GET — para que un atacante con acceso de lectura
	// no pueda firmar webhooks. La UI guarda "se generó" y punto.
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WebhookDelivery struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenantId"`
	EndpointID  *string        `json:"endpointId,omitempty"`
	EventType   string         `json:"eventType"`
	Payload     map[string]any `json:"payload"`
	StatusCode  int            `json:"statusCode"`
	Error       string         `json:"error,omitempty"`
	Attempt     int            `json:"attempt"`
	DeliveredAt *time.Time     `json:"deliveredAt,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type DoNotCallEntry struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Phone     string    `json:"phone"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

type TenantSettings struct {
	TenantID          string    `json:"tenantId"`
	Timezone          string    `json:"timezone"`
	CallerIDDefault   string    `json:"callerIdDefault"`
	AllowedHoursStart string    `json:"allowedHoursStart"`
	AllowedHoursEnd   string    `json:"allowedHoursEnd"`
	AllowedDays       []string  `json:"allowedDays"`
	DailyCallCap      int       `json:"dailyCallCap"`
	RecordingEnabled  bool      `json:"recordingEnabled"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type AuditLogEntry struct {
	ID         string         `json:"id"`
	TenantID   *string        `json:"tenantId,omitempty"`
	ActorID    string         `json:"actorId"`
	ActorEmail string         `json:"actorEmail,omitempty"`
	Action     string         `json:"action"`
	EntityType string         `json:"entityType"`
	EntityID   string         `json:"entityId"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  time.Time      `json:"createdAt"`
}
