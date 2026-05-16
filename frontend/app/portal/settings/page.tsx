"use client";

import { useEffect, useState } from "react";
import { KeyRound, Trash2, UserPlus } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { useToast } from "../../../components/toast";
import { KBPanel } from "../../../components/kb-panel";
import { WebhooksPanel } from "../../../components/webhooks-panel";
import { api, ApiError, TenantSettings, User, VoiceCredentials } from "../../../lib/api";
import { useAuth, useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT } from "../../../lib/i18n";

const TIMEZONES = [
  "Europe/Madrid",
  "Europe/London",
  "Europe/Paris",
  "America/New_York",
  "America/Los_Angeles",
  "America/Mexico_City",
  "America/Bogota",
  "UTC",
];

const WEEKDAY_KEYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] as const;

type SettingsTab = "general" | "team" | "voice" | "kb" | "webhooks" | "security";

const SETTINGS_TABS: { id: SettingsTab; labelKey: string }[] = [
  { id: "general", labelKey: "settings.tab.general" },
  { id: "team", labelKey: "settings.tab.team" },
  { id: "voice", labelKey: "settings.tab.voice" },
  { id: "kb", labelKey: "settings.tab.kb" },
  { id: "webhooks", labelKey: "settings.tab.webhooks" },
  { id: "security", labelKey: "settings.tab.security" },
];

export default function SettingsPage() {
  const { user } = useAuth();
  const tenant = useTenantScope();
  const t = useT();
  const settingsRes = useResource(() => api.tenantSettings(tenant), [tenant]);
  const toast = useToast();

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [tab, setTab] = useState<SettingsTab>("general");

  const canManage = user?.role === "tenant_admin" || user?.role === "platform_admin";

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("settings.title")}</h1>
          <p className="subtle">{t("settings.subtitle.full")}</p>
        </div>
      </div>

      <div className="filter-row" style={{ marginBottom: 16 }}>
        {SETTINGS_TABS.map((tt) => (
          <button
            key={tt.id}
            className={`chip-button${tab === tt.id ? " active" : ""}`}
            onClick={() => setTab(tt.id)}
          >
            {t(tt.labelKey)}
          </button>
        ))}
      </div>

      {tab === "general" ? (
        <>
          <section className="panel">
            <div className="panel-header">
              <div>
                <p className="eyebrow">{t("settings.account.eyebrow")}</p>
                <h2>{user?.name || user?.email}</h2>
              </div>
              <span className="chip">{user?.role}</span>
            </div>
            <div className="form-grid">
              <div className="field">
                <label>{t("login.email")}</label>
                <input value={user?.email ?? ""} readOnly />
              </div>
              <div className="field">
                <label>{t("settings.account.tenant")}</label>
                <input value={user?.tenantId ?? "(platform)"} readOnly />
              </div>
            </div>
          </section>
          <TenantSettingsPanel
            settings={settingsRes.data}
            loading={settingsRes.loading}
            error={settingsRes.error}
            onSaved={() => settingsRes.reload()}
            tenant={tenant}
          />
        </>
      ) : null}

      {tab === "team" ? (
        <TeamPanel tenant={tenant} canManage={canManage} currentUserId={user?.id} />
      ) : null}

      {tab === "voice" ? <VoiceCredentialsPanel tenant={tenant} canManage={canManage} /> : null}

      {tab === "kb" ? <KBPanel /> : null}

      {tab === "webhooks" ? <WebhooksPanel /> : null}

      {tab === "security" ? (
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("settings.security.eyebrow")}</p>
              <h2>{t("settings.security.title")}</h2>
            </div>
          </div>
          <form
            className="form-grid"
            onSubmit={async (event) => {
              event.preventDefault();
              if (next.length < 8) {
                toast.push(t("settings.security.warn.minlen"), "warn");
                return;
              }
              if (next !== confirm) {
                toast.push(t("settings.security.warn.mismatch"), "warn");
                return;
              }
              setSubmitting(true);
              try {
                await api.changePassword(current, next);
                toast.push(t("settings.security.toast.updated"), "success");
                setCurrent("");
                setNext("");
                setConfirm("");
              } catch (err) {
                const code = err instanceof ApiError ? err.code : "error";
                const label = code === "invalid_current_password" ? t("settings.security.toast.invalid_current") : code;
                toast.push(`${t("g.error")}: ${label}`, "danger");
              } finally {
                setSubmitting(false);
              }
            }}
          >
            <div className="field">
              <label>{t("settings.security.current")}</label>
              <input type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
            </div>
            <div className="field">
              <label>{t("settings.security.new")}</label>
              <input type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} minLength={8} required />
            </div>
            <div className="field">
              <label>{t("settings.security.confirm")}</label>
              <input type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} minLength={8} required />
            </div>
            <div className="field" style={{ alignSelf: "end" }}>
              <button className="button" disabled={submitting}>
                {submitting ? t("settings.security.submitting") : t("settings.security.submit")}
              </button>
            </div>
          </form>
          <p className="subtle" style={{ marginTop: 12 }}>{t("settings.security.hint")}</p>
        </section>
      ) : null}
    </>
  );
}

function TeamPanel({ tenant, canManage, currentUserId }: { tenant: string | undefined; canManage: boolean; currentUserId: string | undefined }) {
  const t = useT();
  const team = useResource(() => api.tenantUsers(tenant), [tenant]);
  const toast = useToast();
  const confirm = useConfirm();
  const [formOpen, setFormOpen] = useState(false);
  const [tempPwd, setTempPwd] = useState<{ email: string; pwd: string } | null>(null);

  async function handleInvite(input: { email: string; name: string; role: string }) {
    try {
      const res = await api.inviteTenantUser(input, tenant);
      toast.push(t("settings.team.toast.created", { email: res.user.email }), "success");
      setTempPwd({ email: res.user.email, pwd: res.tempPassword });
      setFormOpen(false);
      team.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("settings.team.toast.create_failed", { err: code }), "danger");
    }
  }

  async function handleRoleChange(u: User, role: string) {
    try {
      await api.updateTenantUserRole(u.id, role, tenant);
      toast.push(t("settings.team.toast.role_updated"), "success");
      team.reload();
    } catch (err) {
      toast.push(t("settings.team.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleDelete(u: User) {
    const ok = await confirm({
      title: t("btn.delete"),
      description: t("settings.team.toast.delete_confirm", { email: u.email }),
      variant: "danger",
      confirmLabel: t("btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteTenantUser(u.id, tenant);
      toast.push(t("settings.team.toast.deleted"), "success");
      team.reload();
    } catch (err) {
      toast.push(t("settings.team.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  const users = team.data ?? [];

  return (
    <section className="panel" style={{ marginTop: 16 }}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">{t("settings.team.eyebrow")}</p>
          <h2>{t("settings.team.title")}</h2>
        </div>
        {canManage ? (
          <button className="button compact" onClick={() => setFormOpen((v) => !v)}>
            <UserPlus aria-hidden="true" />
            <span>{formOpen ? t("settings.team.cancel") : t("settings.team.invite")}</span>
          </button>
        ) : null}
      </div>

      {!canManage ? (
        <p className="subtle">{t("settings.team.needadmin", { role: "tenant_admin" })}</p>
      ) : null}

      {tempPwd ? (
        <div className="panel" style={{ marginBottom: 12, background: "var(--accent-soft)", border: "1px solid var(--accent)" }}>
          <p className="eyebrow">{t("settings.team.temp.eyebrow")}</p>
          <p>
            <strong>{tempPwd.email}</strong> · <code className="mono">{tempPwd.pwd}</code>
          </p>
          <p className="subtle">{t("settings.team.temp.hint")}</p>
          <button className="button secondary compact" onClick={() => setTempPwd(null)}>
            {t("btn.close")}
          </button>
        </div>
      ) : null}

      {formOpen && canManage ? <InviteForm onSubmit={handleInvite} /> : null}

      {team.loading ? (
        <div className="empty-state">{t("settings.team.empty.loading")}</div>
      ) : users.length === 0 ? (
        <div className="empty-state">{t("settings.team.empty")}</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("col.name")}</th>
                <th>{t("login.email")}</th>
                <th>{t("settings.members.role")}</th>
                <th>{t("settings.team.col.lastlogin")}</th>
                <th>{t("settings.team.col.action")}</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const isSelf = u.id === currentUserId;
                return (
                  <tr key={u.id}>
                    <td className="primary-cell">{u.name}{isSelf ? t("settings.team.self") : ""}</td>
                    <td>{u.email}</td>
                    <td>
                      {canManage && !isSelf ? (
                        <select className="inline-select" value={u.role} onChange={(e) => handleRoleChange(u, e.target.value)}>
                          <option value="tenant_admin">tenant_admin</option>
                          <option value="tenant_agent">tenant_agent</option>
                        </select>
                      ) : (
                        <span className="chip">{u.role}</span>
                      )}
                    </td>
                    <td>{u.lastLoginAt ? new Date(u.lastLoginAt).toLocaleString() : "—"}</td>
                    <td>
                      {canManage && !isSelf ? (
                        <button className="button ghost compact" onClick={() => handleDelete(u)}>
                          <Trash2 aria-hidden="true" />
                        </button>
                      ) : null}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function InviteForm({ onSubmit }: { onSubmit: (input: { email: string; name: string; role: string }) => Promise<void> }) {
  const t = useT();
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState("tenant_agent");
  const [submitting, setSubmitting] = useState(false);

  return (
    <form
      className="panel"
      style={{ marginBottom: 12 }}
      onSubmit={async (e) => {
        e.preventDefault();
        setSubmitting(true);
        await onSubmit({ email: email.trim(), name: name.trim(), role });
        setSubmitting(false);
        setEmail("");
        setName("");
        setRole("tenant_agent");
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">{t("settings.invite.eyebrow")}</p>
          <h2>{t("settings.invite.title")}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("col.name")}</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="field">
          <label>{t("login.email")}</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </div>
        <div className="field">
          <label>{t("settings.members.role")}</label>
          <select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="tenant_agent">{t("settings.invite.role.agent")}</option>
            <option value="tenant_admin">{t("settings.invite.role.admin")}</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, justifyContent: "flex-start" }}>
        <button className="button" disabled={submitting}>
          {submitting ? t("settings.invite.submitting") : t("settings.invite.submit")}
        </button>
      </div>
    </form>
  );
}

function TenantSettingsPanel({
  settings,
  loading,
  error,
  onSaved,
  tenant,
}: {
  settings: TenantSettings | null;
  loading: boolean;
  error: string | null;
  onSaved: () => void;
  tenant: string | undefined;
}) {
  const t = useT();
  const toast = useToast();
  const [timezone, setTimezone] = useState("Europe/Madrid");
  const [callerIdDefault, setCallerIdDefault] = useState("");
  const [allowedHoursStart, setAllowedHoursStart] = useState("10:00");
  const [allowedHoursEnd, setAllowedHoursEnd] = useState("18:00");
  const [allowedDays, setAllowedDays] = useState<string[]>(["mon", "tue", "wed", "thu", "fri"]);
  const [dailyCallCap, setDailyCallCap] = useState(250);
  const [recordingEnabled, setRecordingEnabled] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (!settings) return;
    setTimezone(settings.timezone);
    setCallerIdDefault(settings.callerIdDefault);
    setAllowedHoursStart(settings.allowedHoursStart);
    setAllowedHoursEnd(settings.allowedHoursEnd);
    setAllowedDays(settings.allowedDays);
    setDailyCallCap(settings.dailyCallCap);
    setRecordingEnabled(settings.recordingEnabled);
    setDirty(false);
  }, [settings]);

  function toggleDay(day: string) {
    setAllowedDays((prev) => (prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day]));
    setDirty(true);
  }

  async function handleSave(event: React.FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    try {
      await api.updateTenantSettings(
        { timezone, callerIdDefault, allowedHoursStart, allowedHoursEnd, allowedDays, dailyCallCap, recordingEnabled },
        tenant,
      );
      toast.push(t("settings.tenant.toast.saved"), "success");
      onSaved();
    } catch (err) {
      toast.push(t("settings.tenant.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="empty-state">{t("settings.tenant.loading")}</div>
      </section>
    );
  }
  if (error) {
    return (
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="empty-state danger">{t("g.error")}: {error}</div>
      </section>
    );
  }

  return (
    <form className="panel" style={{ marginTop: 16 }} onSubmit={handleSave}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">{t("settings.tenant.eyebrow")}</p>
          <h2>{t("settings.tenant.title")}</h2>
        </div>
        {dirty ? <span className="status warn">{t("settings.tenant.dirty")}</span> : <span className="status good">{t("settings.tenant.synced")}</span>}
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("settings.tenant.timezone")}</label>
          <select
            value={timezone}
            onChange={(e) => {
              setTimezone(e.target.value);
              setDirty(true);
            }}
          >
            {TIMEZONES.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>{t("settings.tenant.callerid")}</label>
          <input
            value={callerIdDefault}
            onChange={(e) => {
              setCallerIdDefault(e.target.value);
              setDirty(true);
            }}
            placeholder="+34 600 000 000"
          />
        </div>
        <div className="field">
          <label>{t("settings.tenant.hours.start")}</label>
          <input
            type="time"
            value={allowedHoursStart}
            onChange={(e) => {
              setAllowedHoursStart(e.target.value);
              setDirty(true);
            }}
          />
        </div>
        <div className="field">
          <label>{t("settings.tenant.hours.end")}</label>
          <input
            type="time"
            value={allowedHoursEnd}
            onChange={(e) => {
              setAllowedHoursEnd(e.target.value);
              setDirty(true);
            }}
          />
        </div>
        <div className="field">
          <label>{t("settings.tenant.dailycap")}</label>
          <input
            type="number"
            min={0}
            value={dailyCallCap}
            onChange={(e) => {
              setDailyCallCap(parseInt(e.target.value, 10) || 0);
              setDirty(true);
            }}
          />
        </div>
        <div className="field">
          <label>{t("settings.tenant.recording")}</label>
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={recordingEnabled}
              onChange={(e) => {
                setRecordingEnabled(e.target.checked);
                setDirty(true);
              }}
            />
            <span>{t("settings.tenant.recording.hint")}</span>
          </label>
        </div>
        <div className="field" style={{ gridColumn: "1 / -1" }}>
          <label>{t("settings.tenant.alloweddays")}</label>
          <div className="filter-row">
            {WEEKDAY_KEYS.map((key) => (
              <button
                type="button"
                key={key}
                className={`chip-button${allowedDays.includes(key) ? " active" : ""}`}
                onClick={() => toggleDay(key)}
              >
                {t(`settings.weekdays.${key}`)}
              </button>
            ))}
          </div>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
        <button className="button" disabled={submitting || !dirty}>
          {submitting ? t("settings.tenant.submitting") : t("settings.tenant.submit")}
        </button>
        <button
          type="button"
          className="button secondary"
          disabled={!dirty}
          onClick={() => {
            if (settings) {
              setTimezone(settings.timezone);
              setCallerIdDefault(settings.callerIdDefault);
              setAllowedHoursStart(settings.allowedHoursStart);
              setAllowedHoursEnd(settings.allowedHoursEnd);
              setAllowedDays(settings.allowedDays);
              setDailyCallCap(settings.dailyCallCap);
              setRecordingEnabled(settings.recordingEnabled);
              setDirty(false);
            }
          }}
        >
          {t("settings.tenant.discard")}
        </button>
      </div>
      {settings ? (
        <p className="subtle" style={{ marginTop: 12 }}>
          {t("settings.tenant.lastupdate", { when: new Date(settings.updatedAt).toLocaleString() })}
        </p>
      ) : null}
    </form>
  );
}

function VoiceCredentialsPanel({ tenant, canManage }: { tenant: string | undefined; canManage: boolean }) {
  const t = useT();
  const creds = useResource(() => api.voiceCredentials(tenant), [tenant]);
  const toast = useToast();
  const [draft, setDraft] = useState<Partial<VoiceCredentials>>({});
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setDraft({});
  }, [tenant, creds.data]);

  if (!canManage) {
    return (
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("settings.voice.eyebrow")}</p>
            <h2>{t("settings.voice.title")}</h2>
          </div>
        </div>
        <p className="subtle">{t("settings.voice.needadmin", { role: "tenant_admin" })}</p>
      </section>
    );
  }

  function setField<K extends keyof VoiceCredentials>(key: K, value: VoiceCredentials[K]) {
    setDraft((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    setSubmitting(true);
    try {
      await api.updateVoiceCredentials(draft, tenant);
      toast.push(t("settings.voice.toast.updated"), "success");
      setDraft({});
      creds.reload();
    } catch (err) {
      toast.push(t("settings.voice.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setSubmitting(false);
    }
  }

  const hasChanges = Object.keys(draft).length > 0;
  const c = creds.data;

  return (
    <section className="panel" style={{ marginTop: 16 }}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">{t("settings.voice.eyebrow")}</p>
          <h2>{t("settings.voice.title.full")}</h2>
        </div>
        <div className="actions">
          {hasChanges ? <span className="status warn">{t("settings.tenant.dirty")}</span> : null}
          <button className="button" disabled={!hasChanges || submitting} onClick={handleSave}>
            <KeyRound aria-hidden="true" />
            <span>{submitting ? t("settings.voice.submitting") : t("settings.voice.submit")}</span>
          </button>
        </div>
      </div>

      <p className="subtle" style={{ marginBottom: 16 }}>{t("settings.voice.hint")}</p>

      <div className="grid two">
        <ProviderBlock
          title="OpenAI Realtime"
          providerId="openai_realtime"
          tenant={tenant}
          subtitle="ASR + LLM + TTS end-to-end. Minimum latency."
          c={c}
          draft={draft}
          fields={[
            { key: "openaiApiKey", label: "API key", type: "password", placeholder: "sk-..." },
            { key: "openaiRealtimeModel", label: "Model", placeholder: "gpt-4o-realtime-preview-2024-12-17" },
            { key: "openaiRealtimeVoice", label: "Voice", placeholder: "alloy" },
          ]}
          setField={setField}
        />
        <ProviderBlock
          title="Deepgram Voice Agent"
          providerId="deepgram"
          tenant={tenant}
          subtitle="wss://agent.deepgram.com — listen + think + speak in a single socket."
          c={c}
          draft={draft}
          fields={[
            { key: "deepgramApiKey", label: "API key Deepgram", type: "password", placeholder: "..." },
            { key: "deepgramListenModel", label: "Listen model (ASR)", placeholder: "nova-3" },
            { key: "deepgramThinkProvider", label: "LLM provider", placeholder: "open_ai · anthropic · ..." },
            { key: "deepgramThinkModel", label: "LLM model", placeholder: "gpt-4o-mini" },
            { key: "deepgramSpeakModel", label: "Speak model (TTS)", placeholder: "aura-2-thalia-en" },
            { key: "deepgramGreeting", label: "Initial greeting (optional)", placeholder: "Hello, I'm the assistant..." },
          ]}
          setField={setField}
        />
        <ProviderBlock
          title="AssemblyAI Voice Agent"
          providerId="assemblyai"
          tenant={tenant}
          subtitle="wss://agents.assemblyai.com — LLM and TTS hosted by AssemblyAI."
          c={c}
          draft={draft}
          fields={[
            { key: "assemblyaiApiKey", label: "API key AssemblyAI", type: "password", placeholder: "..." },
            { key: "assemblyaiVoice", label: "Voice", placeholder: "ivy · james · tyler" },
            { key: "assemblyaiGreeting", label: "Initial greeting (optional)", placeholder: "Hello, I'm the assistant..." },
          ]}
          setField={setField}
        />
      </div>
    </section>
  );
}

function ProviderBlock<K extends keyof VoiceCredentials>({
  title,
  subtitle,
  providerId,
  tenant,
  c,
  draft,
  fields,
  setField,
}: {
  title: string;
  subtitle: string;
  providerId: string;
  tenant: string | undefined;
  c: VoiceCredentials | null;
  draft: Partial<VoiceCredentials>;
  fields: { key: K; label: string; type?: string; placeholder?: string }[];
  setField: <KK extends keyof VoiceCredentials>(key: KK, value: VoiceCredentials[KK]) => void;
}) {
  const t = useT();
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);

  async function handleTest() {
    setTesting(true);
    setTestResult(null);
    try {
      const r = await api.testVoiceCredentials(providerId, tenant);
      setTestResult(
        r.ok
          ? { ok: true, msg: t("settings.voice.test.ok") }
          : { ok: false, msg: r.error ?? t("settings.voice.test.unknownerr") },
      );
    } catch (err) {
      setTestResult({ ok: false, msg: err instanceof ApiError ? err.code : "error" });
    } finally {
      setTesting(false);
    }
  }

  return (
    <div className="panel" style={{ background: "var(--paper)", padding: 18 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12 }}>
        <div>
          <p className="eyebrow">{title}</p>
          <p className="subtle" style={{ marginBottom: 12 }}>{subtitle}</p>
        </div>
        <button type="button" className="button ghost compact" onClick={handleTest} disabled={testing}>
          {testing ? t("settings.voice.testing") : t("settings.voice.test")}
        </button>
      </div>
      {testResult ? (
        <p
          className={testResult.ok ? "status good" : "status danger"}
          style={{ marginBottom: 12, display: "inline-block" }}
        >
          {testResult.ok ? "✓ " : "✗ "}
          {testResult.msg}
        </p>
      ) : null}
      <div style={{ display: "grid", gap: 10 }}>
        {fields.map((f) => {
          const current = draft[f.key] ?? (c ? (c[f.key] as string) : "");
          return (
            <div className="field" key={f.key as string}>
              <label>{f.label}</label>
              <input
                type={f.type ?? "text"}
                value={(current ?? "") as string}
                placeholder={f.placeholder}
                onChange={(e) => setField(f.key, e.target.value as VoiceCredentials[K])}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}
