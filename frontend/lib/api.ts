"use client";

// Same-origin: el browser llama a /api/* del propio portal y Next reenvía al
// backend por la red Docker (ver frontend/next.config.mjs).
export const baseUrl = "";

export type Overview = {
  callsToday: number;
  qualifiedLeads: number;
  callbacks: number;
  activeCampaigns: number;
  queuedCalls: number;
};

export type Tenant = {
  id: string;
  name: string;
  status: string;
  plan: string;
  createdAt: string;
};

export type User = {
  id: string;
  email: string;
  name: string;
  role: string;
  tenantId?: string | null;
  lastLoginAt?: string | null;
  createdAt: string;
};

export type Lead = {
  id: string;
  tenantId: string;
  name: string;
  phone: string;
  email: string;
  type: string;
  status: string;
  source: string;
  consent: string;
  lastActivity: string;
};

export type Property = {
  id: string;
  tenantId: string;
  name: string;
  address: string;
  price: string;
  availability: string;
  requirements: string[];
  faqs: string[];
};

export type Bot = {
  id: string;
  tenantId: string;
  name: string;
  type: string;
  language: string;
  voice: string;
  status: string;
  objective: string;
  guardrails: string[];
  voiceProvider: string;
  didId?: string | null;
  didE164?: string;
  trunkId?: string;
};

export type SIPTrunk = {
  id: string;
  name: string;
  provider: string;
  asteriskEndpoint: string;
  host: string;
  port: number;
  username: string;
  password?: string;
  register: boolean;
  identifyIp: string;
  status: string;
  notes: string;
  didCount: number;
  createdAt: string;
};

export type DID = {
  id: string;
  trunkId: string;
  trunkName?: string;
  asteriskEndpoint?: string;
  tenantId?: string | null;
  e164: string;
  label: string;
  status: string;
  createdAt: string;
};

export type DIDRoutingRule = {
  id: string;
  tenantId: string;
  didId: string;
  name: string;
  priority: number;
  enabled: boolean;
  timezone: string;
  daysOfWeek: number[];
  startMinute?: number | null;
  endMinute?: number | null;
  callerPrefixes: string[];
  language: string;
  targetBotId: string;
  targetBotName?: string;
  fallbackBotId?: string | null;
  fallbackBotName?: string;
  createdAt: string;
  updatedAt: string;
};

export type DIDRoutingRuleInput = {
  name: string;
  priority?: number;
  enabled?: boolean;
  timezone: string;
  daysOfWeek: number[];
  startMinute?: number | null;
  endMinute?: number | null;
  callerPrefixes: string[];
  language: string;
  targetBotId: string;
  fallbackBotId?: string | null;
};

export type DoNotCallEntry = {
  id: string;
  tenantId: string;
  phone: string;
  reason: string;
  createdAt: string;
};

export type VoiceCredentials = {
  tenantId: string;
  openaiApiKey: string;
  openaiRealtimeModel: string;
  openaiRealtimeVoice: string;
  deepgramApiKey: string;
  deepgramListenModel: string;
  deepgramThinkProvider: string;
  deepgramThinkModel: string;
  deepgramSpeakModel: string;
  deepgramGreeting: string;
  assemblyaiApiKey: string;
  assemblyaiVoice: string;
  assemblyaiGreeting: string;
};

export type TenantSettings = {
  tenantId: string;
  timezone: string;
  callerIdDefault: string;
  allowedHoursStart: string;
  allowedHoursEnd: string;
  allowedDays: string[];
  dailyCallCap: number;
  recordingEnabled: boolean;
  updatedAt: string;
};

export type AuditLogEntry = {
  id: string;
  tenantId?: string | null;
  actorId: string;
  actorEmail?: string;
  action: string;
  entityType: string;
  entityId: string;
  payload: Record<string, unknown> | null;
  createdAt: string;
};

export type Campaign = {
  id: string;
  tenantId: string;
  name: string;
  botId: string;
  status: string;
  schedule: string;
  leadCount: number;
  maxAttempts: number;
  retryCooldownMinutes: number;
  startAt?: string | null;
  endAt?: string | null;
  maxConcurrent: number;
};

export type CampaignLead = {
  id: string;
  tenantId: string;
  campaignId: string;
  leadId: string;
  leadName?: string;
  leadPhone?: string;
  status: string;
  attempts: number;
  lastAttemptAt?: string | null;
  outcome: string;
};

export type ImportResult = {
  created: number;
  skipped: number;
  invalid: number;
  errors?: string[];
};

export type Analytics = {
  generatedAt: string;
  timezone: string;
  last7Days: { date: string; count: number }[];
  outcomes: { label: string; count: number }[];
  statuses: { label: string; count: number }[];
  topBots: { label: string; count: number }[];
  topCampaigns: { label: string; count: number }[];
  totalsLast7: number;
  totalsPrev7: number;
  providerSeconds: { provider: string; seconds: number }[];
  costByProvider: {
    provider: string;
    seconds: number;
    centsPerMin: number;
    costCents: number;
  }[];
  totalCostCents: number;
};

export type Call = {
  id: string;
  tenantId: string;
  leadId?: string | null;
  campaignId?: string | null;
  leadName: string;
  phone: string;
  campaign: string;
  status: string;
  outcome: string;
  durationSec: number;
  channelId: string;
  voiceSessionId?: string;
  startedAt?: string | null;
  endedAt?: string | null;
  summary: string;
  recordingUrl?: string;
  provider?: string;
  /** Coste estimado en céntimos USD. Calculado server-side a partir
   *  de provider × duración × tarifa. No se persiste — si cambias
   *  tarifas las llamadas viejas reflejan el nuevo precio. */
  costCents?: number;
};

export type PricingTable = {
  /** Mapa providerId → céntimos por minuto. Ej. { openai_realtime: 30, deepgram: 8 } */
  centsPerMin: Record<string, number>;
};

/** Acciones soportadas para una bot tool. Whitelist replicada del backend. */
export type BotToolActionType =
  | "set_lead_outcome"
  | "set_lead_status"
  | "schedule_callback"
  | "webhook"
  | "end_call"
  | "transfer_human"
  | "search_knowledge_base"
  | "calendar_check_availability"
  | "calendar_schedule_meeting"
  | "calendar_list_my_meetings"
  | "calendar_cancel_meeting"
  | "calendar_reschedule_meeting";

/** Definición de tool en la biblioteca del tenant. */
export type Tool = {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  parametersSchema: Record<string, unknown>;
  actionType: BotToolActionType;
  actionConfig: Record<string, unknown>;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export type ToolInput = {
  name: string;
  description: string;
  parametersSchema: Record<string, unknown>;
  actionType: BotToolActionType;
  actionConfig: Record<string, unknown>;
  enabled?: boolean;
};

/** Vista combinada para el editor del bot: la tool con su estado de asignación. */
export type BotToolView = Tool & {
  assigned: boolean;
  assignedEnabled: boolean;
};

export type WebhookEndpoint = {
  id: string;
  tenantId: string;
  name: string;
  url: string;
  /** Solo presente en la respuesta del POST de create y POST regenerate. */
  secret?: string;
  events: string[];
  active: boolean;
  createdAt: string;
  updatedAt: string;
};

export type WebhookEndpointInput = {
  name: string;
  url: string;
  events: string[];
  active?: boolean;
};

export type CallRecordingMeta = {
  id: string;
  url: string;
  contentType: string;
  sizeBytes: number;
  durationSec: number;
  createdAt: string;
  expiresAt: string;
};

export type CallRecordingListItem = {
  id: string;
  callId: string;
  tenantId: string;
  storageKey: string;
  contentType: string;
  sizeBytes: number;
  durationSec: number;
  status: string;
  retentionDueAt?: string;
  createdAt: string;
  leadName: string;
  phone: string;
  campaign: string;
  outcome: string;
  url: string;
};

export type RecordingsPage = {
  items: CallRecordingListItem[];
  total: number;
  page: number;
  pageSize: number;
};

export type RecordingUsage = {
  totalBytes: number;
  count: number;
  oldestAt?: string;
};

export type KBDocument = {
  id: string;
  tenantId: string;
  name: string;
  mimeType: string;
  sizeBytes: number;
  status: "pending" | "processing" | "ready" | "failed";
  error?: string;
  chunkCount: number;
  createdAt: string;
  updatedAt: string;
};

export type KBSearchHit = {
  chunk: string;
  document: string;
  score: number;
};

export type WebhookDelivery = {
  id: string;
  tenantId: string;
  endpointId?: string;
  eventType: string;
  payload: Record<string, unknown>;
  statusCode: number;
  error?: string;
  attempt: number;
  deliveredAt?: string;
  createdAt: string;
};

export type LoginResponse = {
  token: string;
  expiresAt: string;
  user: User;
};

export type TestCallResponse = {
  call: Call;
  channel?: { id: string; state: string; name: string };
  endpoint?: string;
  message?: string;
  voiceSessionId?: string;
};

const TOKEN_KEY = "callhub_token";
const USER_KEY = "callhub_user";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

export function getStoredUser(): User | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function setSession(token: string, user: User) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(TOKEN_KEY, token);
  window.localStorage.setItem(USER_KEY, JSON.stringify(user));
}

export function clearSession() {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(TOKEN_KEY);
  window.localStorage.removeItem(USER_KEY);
}

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string) {
    super(`API ${status}: ${code}`);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(method: string, path: string, body?: unknown, rawBody?: { body: BodyInit; contentType: string }): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  let payload: BodyInit | undefined;
  if (rawBody) {
    headers["Content-Type"] = rawBody.contentType;
    payload = rawBody.body;
  } else if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }

  const response = await fetch(`${baseUrl}${path}`, {
    method,
    headers,
    body: payload,
    cache: "no-store",
  });

  if (response.status === 204) return undefined as T;
  if (!response.ok) {
    let code = response.statusText || "error";
    try {
      const data = await response.json();
      if (data && typeof data.error === "string") code = data.error;
    } catch {
      /* ignore */
    }
    if (response.status === 401) clearSession();
    throw new ApiError(response.status, code);
  }
  return response.json() as Promise<T>;
}

export const api = {
  login: (email: string, password: string) =>
    request<LoginResponse>("POST", "/api/auth/login", { email, password }),
  me: () => request<User>("GET", "/api/auth/me"),
  changePassword: (current: string, next: string) =>
    request<void>("POST", "/api/auth/password", { current, new: next }),
  overview: (tenantOverride?: string) => request<Overview>("GET", withTenant("/api/overview", tenantOverride)),
  tenants: () => request<Tenant[]>("GET", "/api/admin/tenants"),
  operations: () => request<Record<string, unknown>>("GET", "/api/admin/operations"),

  leads: (tenantOverride?: string) => request<Lead[]>("GET", withTenant("/api/leads", tenantOverride)),
  getLead: (id: string, tenantOverride?: string) =>
    request<Lead>("GET", withTenant(`/api/leads/${encodeURIComponent(id)}`, tenantOverride)),
  leadCalls: (id: string, tenantOverride?: string) =>
    request<Call[]>("GET", withTenant(`/api/leads/${encodeURIComponent(id)}/calls`, tenantOverride)),
  analytics: (tenantOverride?: string) =>
    request<Analytics>("GET", withTenant("/api/analytics", tenantOverride)),
  pricing: () => request<PricingTable>("GET", "/api/pricing"),
  // Biblioteca de tools por tenant (vive en /portal/tools).
  tools: (tenantOverride?: string) =>
    request<Tool[]>("GET", withTenant("/api/tools", tenantOverride)),
  createTool: (input: ToolInput, tenantOverride?: string) =>
    request<Tool>("POST", withTenant("/api/tools", tenantOverride), input),
  updateTool: (id: string, input: ToolInput, tenantOverride?: string) =>
    request<Tool>("PATCH", withTenant(`/api/tools/${id}`, tenantOverride), input),
  deleteTool: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/tools/${id}`, tenantOverride)),
  // Asignaciones bot ↔ tool.
  botToolAssignments: (botId: string, tenantOverride?: string) =>
    request<BotToolView[]>("GET", withTenant(`/api/bots/${botId}/tools`, tenantOverride)),
  assignToolToBot: (botId: string, toolId: string, enabled: boolean, tenantOverride?: string) =>
    request<void>("PUT", withTenant(`/api/bots/${botId}/tools/${toolId}`, tenantOverride), { enabled }),
  unassignToolFromBot: (botId: string, toolId: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/bots/${botId}/tools/${toolId}`, tenantOverride)),
  webhooks: (tenantOverride?: string) =>
    request<WebhookEndpoint[]>("GET", withTenant("/api/webhooks", tenantOverride)),
  createWebhook: (input: WebhookEndpointInput, tenantOverride?: string) =>
    request<WebhookEndpoint>("POST", withTenant("/api/webhooks", tenantOverride), input),
  updateWebhook: (id: string, input: WebhookEndpointInput, tenantOverride?: string) =>
    request<WebhookEndpoint>("PATCH", withTenant(`/api/webhooks/${id}`, tenantOverride), input),
  deleteWebhook: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/webhooks/${id}`, tenantOverride)),
  regenerateWebhookSecret: (id: string, tenantOverride?: string) =>
    request<{ secret: string }>("POST", withTenant(`/api/webhooks/${id}/regenerate`, tenantOverride)),
  webhookDeliveries: (tenantOverride?: string) =>
    request<WebhookDelivery[]>("GET", withTenant("/api/webhook-deliveries", tenantOverride)),
  webhookEvents: () => request<{ events: string[] }>("GET", "/api/webhook-events"),
  kbDocuments: (tenantOverride?: string) =>
    request<KBDocument[]>("GET", withTenant("/api/kb/documents", tenantOverride)),
  uploadKBDocument: async (file: File, tenantOverride?: string): Promise<KBDocument> => {
    // Multipart upload manual — request() asume JSON. Mantenemos JWT y
    // tenant override en query string como el resto del cliente.
    const fd = new FormData();
    fd.append("file", file);
    const url = withTenant("/api/kb/documents", tenantOverride);
    const token = getToken();
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const resp = await fetch(url, { method: "POST", headers, body: fd });
    if (!resp.ok) {
      let code = `http_${resp.status}`;
      try {
        const body = await resp.json();
        code = body.error || code;
      } catch {
        /* keep generic */
      }
      throw new ApiError(resp.status, code);
    }
    return resp.json();
  },
  deleteKBDocument: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/kb/documents/${id}`, tenantOverride)),
  kbSearch: (q: string, tenantOverride?: string) =>
    request<KBSearchHit[]>(
      "GET",
      withTenant(`/api/kb/search?q=${encodeURIComponent(q)}`, tenantOverride)
    ),
  callRecording: (callId: string, tenantOverride?: string) =>
    request<CallRecordingMeta>("GET", withTenant(`/api/calls/${callId}/recording`, tenantOverride)),
  recordings: (params: {
    page?: number;
    pageSize?: number;
    outcome?: string;
    from?: string;
    to?: string;
  } = {}, tenantOverride?: string) => {
    const q = new URLSearchParams();
    if (params.page) q.set("page", String(params.page));
    if (params.pageSize) q.set("pageSize", String(params.pageSize));
    if (params.outcome) q.set("outcome", params.outcome);
    if (params.from) q.set("from", params.from);
    if (params.to) q.set("to", params.to);
    const qs = q.toString();
    return request<RecordingsPage>(
      "GET",
      withTenant(`/api/recordings${qs ? "?" + qs : ""}`, tenantOverride)
    );
  },
  deleteRecording: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/recordings/${id}`, tenantOverride)),
  recordingsUsage: (tenantOverride?: string) =>
    request<RecordingUsage>("GET", withTenant("/api/recordings/usage", tenantOverride)),
  calendarStatus: (botId: string, tenantOverride?: string) =>
    request<{
      connected: boolean;
      provider?: string;
      accountEmail?: string;
      connectedAt?: string;
    }>("GET", withTenant(`/api/bots/${botId}/calendar`, tenantOverride)),
  calendarAuthorize: (botId: string, tenantOverride?: string) =>
    request<{ authUrl: string }>(
      "POST",
      withTenant(`/api/bots/${botId}/calendar/authorize`, tenantOverride)
    ),
  calendarDisconnect: (botId: string, tenantOverride?: string) =>
    request<void>(
      "DELETE",
      withTenant(`/api/bots/${botId}/calendar`, tenantOverride)
    ),
  createLead: (input: Partial<Lead>, tenantOverride?: string) =>
    request<Lead>("POST", withTenant("/api/leads", tenantOverride), input),
  importLeads: (csv: string, tenantOverride?: string) =>
    request<ImportResult>("POST", withTenant("/api/leads/import", tenantOverride), undefined, {
      body: csv,
      contentType: "text/csv",
    }),
  updateLead: (id: string, patch: Partial<Lead>, tenantOverride?: string) =>
    request<Lead>("PATCH", withTenant(`/api/leads/${encodeURIComponent(id)}`, tenantOverride), patch),
  deleteLead: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/leads/${encodeURIComponent(id)}`, tenantOverride)),

  properties: (tenantOverride?: string) => request<Property[]>("GET", withTenant("/api/properties", tenantOverride)),
  createProperty: (input: Partial<Property>, tenantOverride?: string) =>
    request<Property>("POST", withTenant("/api/properties", tenantOverride), input),
  updateProperty: (id: string, patch: Partial<Property>, tenantOverride?: string) =>
    request<void>("PATCH", withTenant(`/api/properties/${encodeURIComponent(id)}`, tenantOverride), patch),
  deleteProperty: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/properties/${encodeURIComponent(id)}`, tenantOverride)),

  bots: (tenantOverride?: string) => request<Bot[]>("GET", withTenant("/api/bots", tenantOverride)),
  createBot: (input: Partial<Bot>, tenantOverride?: string) =>
    request<Bot>("POST", withTenant("/api/bots", tenantOverride), input),
  updateBot: (id: string, patch: Partial<Bot>, tenantOverride?: string) =>
    request<Bot>("PATCH", withTenant(`/api/bots/${encodeURIComponent(id)}`, tenantOverride), patch),
  deleteBot: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/bots/${encodeURIComponent(id)}`, tenantOverride)),
  assignBotDID: (botId: string, didId: string | null, tenantOverride?: string) =>
    request<Bot>("POST", withTenant(`/api/bots/${encodeURIComponent(botId)}/did`, tenantOverride), { didId }),
  myDIDs: (tenantOverride?: string) => request<DID[]>("GET", withTenant("/api/dids", tenantOverride)),

  campaigns: (tenantOverride?: string) => request<Campaign[]>("GET", withTenant("/api/campaigns", tenantOverride)),
  createCampaign: (input: Partial<Campaign>, tenantOverride?: string) =>
    request<Campaign>("POST", withTenant("/api/campaigns", tenantOverride), input),
  updateCampaign: (id: string, patch: Partial<Campaign>, tenantOverride?: string) =>
    request<Campaign>("PATCH", withTenant(`/api/campaigns/${encodeURIComponent(id)}`, tenantOverride), patch),
  deleteCampaign: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/campaigns/${encodeURIComponent(id)}`, tenantOverride)),
  campaignLeads: (id: string, tenantOverride?: string) =>
    request<CampaignLead[]>("GET", withTenant(`/api/campaigns/${encodeURIComponent(id)}/leads`, tenantOverride)),
  addCampaignLeads: (id: string, leadIds: string[], tenantOverride?: string) =>
    request<{ created: number; total: number }>(
      "POST",
      withTenant(`/api/campaigns/${encodeURIComponent(id)}/leads`, tenantOverride),
      { leadIds },
    ),
  removeCampaignLead: (id: string, leadId: string, tenantOverride?: string) =>
    request<void>(
      "DELETE",
      withTenant(`/api/campaigns/${encodeURIComponent(id)}/leads/${encodeURIComponent(leadId)}`, tenantOverride),
    ),

  calls: (tenantOverride?: string) => request<Call[]>("GET", withTenant("/api/calls", tenantOverride)),
  testCall: (input: { phone: string; leadName?: string; botId?: string }) =>
    request<TestCallResponse>("POST", "/api/calls/test", input),

  dnc: (tenantOverride?: string) => request<DoNotCallEntry[]>("GET", withTenant("/api/dnc", tenantOverride)),
  addDNC: (input: { phone: string; reason?: string }, tenantOverride?: string) =>
    request<DoNotCallEntry>("POST", withTenant("/api/dnc", tenantOverride), input),
  removeDNC: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/dnc/${encodeURIComponent(id)}`, tenantOverride)),

  audit: (tenantOverride?: string) =>
    request<AuditLogEntry[]>("GET", withTenant("/api/audit", tenantOverride)),

  tenantSettings: (tenantOverride?: string) =>
    request<TenantSettings>("GET", withTenant("/api/tenant/settings", tenantOverride)),
  updateTenantSettings: (patch: Partial<TenantSettings>, tenantOverride?: string) =>
    request<TenantSettings>("PATCH", withTenant("/api/tenant/settings", tenantOverride), patch),

  voiceCredentials: (tenantOverride?: string) =>
    request<VoiceCredentials>("GET", withTenant("/api/tenant/voice-credentials", tenantOverride)),
  updateVoiceCredentials: (patch: Partial<VoiceCredentials>, tenantOverride?: string) =>
    request<VoiceCredentials>("PATCH", withTenant("/api/tenant/voice-credentials", tenantOverride), patch),
  testVoiceCredentials: (provider: string, tenantOverride?: string) =>
    request<{ ok: boolean; error?: string }>(
      "POST",
      withTenant("/api/tenant/voice-credentials/test", tenantOverride),
      { provider },
    ),
  voiceCatalog: () =>
    request<{ providers: { id: string; label: string; models: { id: string; label: string }[]; voices: { id: string; label: string }[]; extraFields?: string[] }[] }>(
      "GET",
      "/api/voice-catalog",
    ),

  tenantUsers: (tenantOverride?: string) =>
    request<User[]>("GET", withTenant("/api/tenant/users", tenantOverride)),
  inviteTenantUser: (input: { email: string; name: string; role: string }, tenantOverride?: string) =>
    request<{ user: User; tempPassword: string }>("POST", withTenant("/api/tenant/users", tenantOverride), input),
  updateTenantUserRole: (id: string, role: string, tenantOverride?: string) =>
    request<void>("PATCH", withTenant(`/api/tenant/users/${encodeURIComponent(id)}`, tenantOverride), { role }),
  deleteTenantUser: (id: string, tenantOverride?: string) =>
    request<void>("DELETE", withTenant(`/api/tenant/users/${encodeURIComponent(id)}`, tenantOverride)),

  getCall: (id: string, tenantOverride?: string) =>
    request<Call>("GET", withTenant(`/api/calls/${encodeURIComponent(id)}`, tenantOverride)),
  callTranscripts: (id: string, tenantOverride?: string) =>
    request<{ id: string; callId: string; role: string; text: string; occurredAt: string }[]>(
      "GET",
      withTenant(`/api/calls/${encodeURIComponent(id)}/transcripts`, tenantOverride),
    ),

  // Admin
  adminCreateTenant: (input: { id: string; name: string; plan?: string; status?: string }) =>
    request<Tenant>("POST", "/api/admin/tenants", input),
  adminUpdateTenant: (id: string, input: { name?: string; plan?: string; status?: string }) =>
    request<void>("PATCH", `/api/admin/tenants/${encodeURIComponent(id)}`, input),
  adminTrunks: () => request<SIPTrunk[]>("GET", "/api/admin/trunks"),
  adminTrunkStatus: () =>
    request<{ ariEnabled: boolean; endpoints: { technology: string; resource: string; state: string; channel_ids: string[] }[] }>(
      "GET",
      "/api/admin/trunks/status",
    ),
  adminCreateTrunk: (input: Partial<SIPTrunk>) => request<SIPTrunk>("POST", "/api/admin/trunks", input),
  adminUpdateTrunk: (id: string, input: Partial<SIPTrunk>) =>
    request<void>("PATCH", `/api/admin/trunks/${encodeURIComponent(id)}`, input),
  adminDeleteTrunk: (id: string) => request<void>("DELETE", `/api/admin/trunks/${encodeURIComponent(id)}`),
  adminDIDs: (tenantFilter?: string) =>
    request<DID[]>("GET", tenantFilter ? `/api/admin/dids?tenant=${encodeURIComponent(tenantFilter)}` : "/api/admin/dids"),
  adminCreateDID: (input: { trunkId: string; e164: string; label?: string; tenantId?: string | null; status?: string }) =>
    request<DID>("POST", "/api/admin/dids", input),
  adminUpdateDID: (id: string, input: { e164: string; label?: string; status?: string }) =>
    request<void>("PATCH", `/api/admin/dids/${encodeURIComponent(id)}`, input),
  adminAssignDID: (id: string, tenantId: string | null) =>
    request<void>("PATCH", `/api/admin/dids/${encodeURIComponent(id)}/assign`, { tenantId }),
  adminDeleteDID: (id: string) => request<void>("DELETE", `/api/admin/dids/${encodeURIComponent(id)}`),
  adminDIDRoutingRules: (didId: string) =>
    request<DIDRoutingRule[]>("GET", `/api/admin/dids/${encodeURIComponent(didId)}/routing-rules`),
  adminCreateDIDRoutingRule: (didId: string, input: DIDRoutingRuleInput) =>
    request<DIDRoutingRule>("POST", `/api/admin/dids/${encodeURIComponent(didId)}/routing-rules`, input),
  adminUpdateDIDRoutingRule: (didId: string, ruleId: string, input: DIDRoutingRuleInput) =>
    request<DIDRoutingRule>(
      "PATCH",
      `/api/admin/dids/${encodeURIComponent(didId)}/routing-rules/${encodeURIComponent(ruleId)}`,
      input,
    ),
  adminDeleteDIDRoutingRule: (didId: string, ruleId: string) =>
    request<void>(
      "DELETE",
      `/api/admin/dids/${encodeURIComponent(didId)}/routing-rules/${encodeURIComponent(ruleId)}`,
    ),
  adminAudit: (tenantFilter?: string) =>
    request<AuditLogEntry[]>("GET", tenantFilter ? `/api/admin/audit?tenant=${encodeURIComponent(tenantFilter)}` : "/api/admin/audit"),
};

function withTenant(path: string, tenant?: string): string {
  if (!tenant) return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}tenant=${encodeURIComponent(tenant)}`;
}

// formatCostCents convierte céntimos (entero) a una string legible en USD.
// Ej. 15 → "$0.15", 1234 → "$12.34", 0 → "—". Centramos USD porque las
// tarifas de los providers (OpenAI/Deepgram/AssemblyAI) están en USD; si
// queremos mostrar EUR habría que multiplicar por un tipo de cambio fijo
// o variable. Para "estimate" en el dashboard sobra con USD.
// formatBytes muestra GB/MB/KB con 1 decimal. Lo usamos para "Storage
// usado: 3.2 GB" en el dashboard y "5.4 MB" en la lista de grabaciones.
export function formatBytes(n: number | undefined): string {
  if (!n || n <= 0) return "0 B";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

// formatDurationShort: 65s → "1:05", 3725s → "1h 2:05". Para columnas
// de duración compactas en listings.
export function formatDurationShort(sec: number | undefined): string {
  if (!sec || sec <= 0) return "—";
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  const ss = s.toString().padStart(2, "0");
  if (h > 0) return `${h}h ${m}:${ss}`;
  return `${m}:${ss}`;
}

export function formatCostCents(cents: number | undefined): string {
  if (cents === undefined || cents === null) return "—";
  if (cents === 0) return "—";
  return `$${(cents / 100).toFixed(2)}`;
}

export function statusClass(status: string) {
  if (["active", "completed", "qualified", "answered", "registrado"].includes(status)) {
    return "status good";
  }
  if (["paused", "callback", "queued", "draft", "dialing", "calling"].includes(status)) {
    return "status warn";
  }
  if (["failed", "blocked", "no_answer", "busy", "unreachable", "caído"].includes(status)) {
    return "status danger";
  }
  return "status";
}

// Mapa de status técnicos a etiquetas para usuario final. La idea es que en la
// UI nunca aparezcan "queued", "dialing", "no_answer", etc. crudos en inglés.
// Cualquier valor no mapeado se devuelve sin cambio (mejor que romper) — si
// aparece, añadirlo aquí.
const STATUS_LABELS: Record<string, string> = {
  // Llamadas / campaigns
  active: "Activa",
  paused: "Pausada",
  draft: "Borrador",
  scheduled: "Programada",
  completed: "Completada",
  queued: "En cola",
  dialing: "Marcando",
  calling: "Llamando",
  in_progress: "En curso",
  answered: "Contestada",
  failed: "Fallida",
  no_answer: "Sin respuesta",
  busy: "Ocupado",
  unreachable: "Inalcanzable",
  blocked: "Bloqueado",
  pending: "Pendiente",
  skipped: "Omitida",
  // Outcomes
  qualified: "Cualificado",
  callback: "Llamar de vuelta",
  // Leads / DIDs / trunks / users
  new: "Nuevo",
  disabled: "Deshabilitado",
  // SIP
  registrado: "Registrado",
  desconocido: "Desconocido",
  online: "En línea",
  offline: "Sin conexión",
};

export function statusLabel(status: string): string {
  if (!status) return "—";
  return STATUS_LABELS[status] || status;
}
