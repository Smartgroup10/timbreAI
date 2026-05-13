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
  createLead: (input: Partial<Lead>) => request<Lead>("POST", "/api/leads", input),
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
  adminAudit: (tenantFilter?: string) =>
    request<AuditLogEntry[]>("GET", tenantFilter ? `/api/admin/audit?tenant=${encodeURIComponent(tenantFilter)}` : "/api/admin/audit"),
};

function withTenant(path: string, tenant?: string): string {
  if (!tenant) return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}tenant=${encodeURIComponent(tenant)}`;
}

export function statusClass(status: string) {
  if (["active", "completed", "qualified", "scheduled", "answered"].includes(status)) {
    return "status good";
  }
  if (["paused", "callback", "queued", "draft", "dialing"].includes(status)) {
    return "status warn";
  }
  if (["failed", "blocked", "no_answer", "busy"].includes(status)) {
    return "status danger";
  }
  return "status";
}
