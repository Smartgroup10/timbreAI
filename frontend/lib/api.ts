const baseUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
};

export type Call = {
  id: string;
  tenantId: string;
  leadName: string;
  phone: string;
  campaign: string;
  status: string;
  outcome: string;
  durationSec: number;
  startedAt: string;
  summary: string;
};

async function get<T>(path: string): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`API error ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export const api = {
  overview: () => get<Overview>("/api/overview"),
  tenants: () => get<Tenant[]>("/api/admin/tenants"),
  leads: () => get<Lead[]>("/api/leads"),
  properties: () => get<Property[]>("/api/properties"),
  bots: () => get<Bot[]>("/api/bots"),
  campaigns: () => get<Campaign[]>("/api/campaigns"),
  calls: () => get<Call[]>("/api/calls")
};

export function statusClass(status: string) {
  if (["active", "completed", "qualified", "scheduled"].includes(status)) {
    return "status good";
  }
  if (["paused", "callback", "queued", "draft"].includes(status)) {
    return "status warn";
  }
  return "status";
}

