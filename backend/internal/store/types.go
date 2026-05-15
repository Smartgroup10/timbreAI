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

// ScheduledMeeting es una cita creada por el bot vía
// calendar_schedule_meeting. La persistimos local para validar
// ownership cuando el lead pida cancelar o mover — sin esto cualquiera
// con el event_id podría tocar la cita de otro.
type ScheduledMeeting struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenantId"`
	BotID           string    `json:"botId"`
	LeadID          *string   `json:"leadId,omitempty"`
	LeadPhone       string    `json:"leadPhone"`
	Provider        string    `json:"provider"`
	ProviderEventID string    `json:"providerEventId"`
	CalendarID      string    `json:"calendarId"`
	HTMLLink        string    `json:"htmlLink,omitempty"`
	Title           string    `json:"title"`
	StartAt         time.Time `json:"startAt"`
	EndAt           time.Time `json:"endAt"`
	AttendeeEmail   string    `json:"attendeeEmail,omitempty"`
	Status          string    `json:"status"` // scheduled | cancelled
	CreatedCallID   *string   `json:"createdCallId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// BotCalendarIntegration es la conexión OAuth de un bot con Google
// Calendar (otros providers en el futuro). Los tokens se guardan
// cifrados y no se exponen al JSON de salida — el frontend solo ve
// si está conectado y con qué email.
type BotCalendarIntegration struct {
	ID                   string     `json:"id"`
	TenantID             string     `json:"tenantId"`
	BotID                string     `json:"botId"`
	Provider             string     `json:"provider"`
	AccountEmail         string     `json:"accountEmail"`
	CalendarID           string     `json:"calendarId"`
	Scopes               string     `json:"scopes,omitempty"`
	ConnectedAt          time.Time  `json:"connectedAt"`
	LastUsedAt           *time.Time `json:"lastUsedAt,omitempty"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	// Tokens internos — nunca se serializan al cliente.
	RefreshTokenPlain    string     `json:"-"`
	AccessTokenPlain     string     `json:"-"`
	AccessTokenExpiresAt *time.Time `json:"-"`
}

// KBDocument representa un documento subido a la knowledge base del
// tenant. El contenido original no se almacena después del chunking —
// vive solo en disco del operador. Lo que persiste son los chunks +
// embeddings (vector DB).
type KBDocument struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenantId"`
	Name       string    `json:"name"`
	MimeType   string    `json:"mimeType"`
	SizeBytes  int64     `json:"sizeBytes"`
	Status     string    `json:"status"` // pending / processing / ready / failed
	Error      string    `json:"error,omitempty"`
	ChunkCount int       `json:"chunkCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// KBChunk es un fragmento con embedding. Solo se usa para INSERT — el
// search devuelve KBSearchHit con score.
type KBChunk struct {
	ID         string
	TenantID   string
	DocumentID string
	ChunkIndex int
	Content    string
	Tokens     int
	Embedding  []float32
}

// KBSearchHit es un chunk + su similitud con la query. Score = 1 - cosine_distance,
// rango [0, 1] donde 1 es match perfecto.
type KBSearchHit struct {
	Chunk    string  `json:"chunk"`
	Document string  `json:"document"`
	Score    float64 `json:"score"`
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
	TenantID               string    `json:"tenantId"`
	Timezone               string    `json:"timezone"`
	CallerIDDefault        string    `json:"callerIdDefault"`
	AllowedHoursStart      string    `json:"allowedHoursStart"`
	AllowedHoursEnd        string    `json:"allowedHoursEnd"`
	AllowedDays            []string  `json:"allowedDays"`
	DailyCallCap           int       `json:"dailyCallCap"`
	RecordingEnabled       bool      `json:"recordingEnabled"`
	// 0 = guardar indefinido. >0 = borrar grabaciones N días después
	// de crearlas (soft delete + objeto en MinIO).
	RecordingRetentionDays int       `json:"recordingRetentionDays"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// CallRecording es una grabación almacenada en MinIO. NUNCA expone la
// URL — el JSON solo trae el ID, key, tamaño y duración. La presigned
// URL se genera on-demand al servir /api/calls/:id/recording.
type CallRecording struct {
	ID             string     `json:"id"`
	CallID         string     `json:"callId"`
	TenantID       string     `json:"tenantId"`
	StorageKey     string     `json:"storageKey"`
	ContentType    string     `json:"contentType"`
	SizeBytes      int64      `json:"sizeBytes"`
	DurationSec    int        `json:"durationSec"`
	Status         string     `json:"status"` // available | archived
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	RetentionDueAt *time.Time `json:"retentionDueAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// CallRecordingListItem agrega info de la call para el listing — evita
// que la UI tenga que cargar la call separadamente para cada recording.
type CallRecordingListItem struct {
	CallRecording
	LeadName string `json:"leadName"`
	Phone    string `json:"phone"`
	Campaign string `json:"campaign"`
	Outcome  string `json:"outcome"`
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
