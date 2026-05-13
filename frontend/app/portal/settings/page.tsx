"use client";

import { useEffect, useState } from "react";
import { KeyRound, Trash2, UserPlus } from "lucide-react";
import { useToast } from "../../../components/toast";
import { api, ApiError, TenantSettings, User, VoiceCredentials } from "../../../lib/api";
import { useAuth, useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

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

const WEEKDAYS: { key: string; label: string }[] = [
  { key: "mon", label: "Lun" },
  { key: "tue", label: "Mar" },
  { key: "wed", label: "Mié" },
  { key: "thu", label: "Jue" },
  { key: "fri", label: "Vie" },
  { key: "sat", label: "Sáb" },
  { key: "sun", label: "Dom" },
];

export default function SettingsPage() {
  const { user } = useAuth();
  const tenant = useTenantScope();
  const settingsRes = useResource(() => api.tenantSettings(tenant), [tenant]);
  const toast = useToast();

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Configuración</h1>
          <p className="subtle">Parámetros de cuenta, telefonía y horarios de operación.</p>
        </div>
      </div>

      <section className="panel">
        <div className="panel-header">
          <div>
            <p className="eyebrow">Tu cuenta</p>
            <h2>{user?.name || user?.email}</h2>
          </div>
          <span className="chip">{user?.role}</span>
        </div>
        <div className="form-grid">
          <div className="field">
            <label>Email</label>
            <input value={user?.email ?? ""} readOnly />
          </div>
          <div className="field">
            <label>Tenant</label>
            <input value={user?.tenantId ?? "(platform)"} readOnly />
          </div>
        </div>
      </section>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Seguridad</p>
            <h2>Cambiar contraseña</h2>
          </div>
        </div>
        <form
          className="form-grid"
          onSubmit={async (event) => {
            event.preventDefault();
            if (next.length < 8) {
              toast.push("La nueva contraseña debe tener al menos 8 caracteres", "warn");
              return;
            }
            if (next !== confirm) {
              toast.push("La confirmación no coincide", "warn");
              return;
            }
            setSubmitting(true);
            try {
              await api.changePassword(current, next);
              toast.push("Contraseña actualizada", "success");
              setCurrent("");
              setNext("");
              setConfirm("");
            } catch (err) {
              const code = err instanceof ApiError ? err.code : "error";
              const label = code === "invalid_current_password" ? "La contraseña actual no es correcta" : code;
              toast.push(`Error: ${label}`, "danger");
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <div className="field">
            <label>Contraseña actual</label>
            <input type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
          </div>
          <div className="field">
            <label>Nueva contraseña</label>
            <input type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} minLength={8} required />
          </div>
          <div className="field">
            <label>Confirmar nueva contraseña</label>
            <input type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} minLength={8} required />
          </div>
          <div className="field" style={{ alignSelf: "end" }}>
            <button className="button" disabled={submitting}>
              {submitting ? "Guardando…" : "Actualizar contraseña"}
            </button>
          </div>
        </form>
        <p className="subtle" style={{ marginTop: 12 }}>
          Mínimo 8 caracteres. Los tokens emitidos antes del cambio siguen siendo válidos hasta su expiración natural.
        </p>
      </section>

      <TenantSettingsPanel settings={settingsRes.data} loading={settingsRes.loading} error={settingsRes.error} onSaved={() => settingsRes.reload()} tenant={tenant} />

      <VoiceCredentialsPanel tenant={tenant} canManage={user?.role === "tenant_admin" || user?.role === "platform_admin"} />

      <TeamPanel tenant={tenant} canManage={user?.role === "tenant_admin" || user?.role === "platform_admin"} currentUserId={user?.id} />
    </>
  );
}

function TeamPanel({ tenant, canManage, currentUserId }: { tenant: string | undefined; canManage: boolean; currentUserId: string | undefined }) {
  const team = useResource(() => api.tenantUsers(tenant), [tenant]);
  const toast = useToast();
  const [formOpen, setFormOpen] = useState(false);
  const [tempPwd, setTempPwd] = useState<{ email: string; pwd: string } | null>(null);

  async function handleInvite(input: { email: string; name: string; role: string }) {
    try {
      const res = await api.inviteTenantUser(input, tenant);
      toast.push(`Usuario ${res.user.email} creado`, "success");
      setTempPwd({ email: res.user.email, pwd: res.tempPassword });
      setFormOpen(false);
      team.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(`No se pudo crear: ${code}`, "danger");
    }
  }

  async function handleRoleChange(u: User, role: string) {
    try {
      await api.updateTenantUserRole(u.id, role, tenant);
      toast.push("Rol actualizado", "success");
      team.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleDelete(u: User) {
    if (!confirm(`Eliminar a ${u.email}? Perderá acceso inmediatamente.`)) return;
    try {
      await api.deleteTenantUser(u.id, tenant);
      toast.push("Usuario eliminado", "success");
      team.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  const users = team.data ?? [];

  return (
    <section className="panel" style={{ marginTop: 16 }}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">Equipo</p>
          <h2>Miembros del tenant</h2>
        </div>
        {canManage ? (
          <button className="button compact" onClick={() => setFormOpen((v) => !v)}>
            <UserPlus aria-hidden="true" />
            <span>{formOpen ? "Cancelar" : "Invitar"}</span>
          </button>
        ) : null}
      </div>

      {!canManage ? (
        <p className="subtle">Necesitas rol <code>tenant_admin</code> para gestionar miembros.</p>
      ) : null}

      {tempPwd ? (
        <div className="panel" style={{ marginBottom: 12, background: "var(--accent-soft)", border: "1px solid var(--accent)" }}>
          <p className="eyebrow">Contraseña temporal</p>
          <p>
            <strong>{tempPwd.email}</strong> · <code className="mono">{tempPwd.pwd}</code>
          </p>
          <p className="subtle">
            Compártela por un canal seguro. El usuario debería cambiarla en Configuración después de su primer login.
          </p>
          <button className="button secondary compact" onClick={() => setTempPwd(null)}>
            Cerrar
          </button>
        </div>
      ) : null}

      {formOpen && canManage ? <InviteForm onSubmit={handleInvite} /> : null}

      {team.loading ? (
        <div className="empty-state">Cargando…</div>
      ) : users.length === 0 ? (
        <div className="empty-state">No hay miembros en este tenant aún.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Nombre</th>
                <th>Email</th>
                <th>Rol</th>
                <th>Último acceso</th>
                <th>Acción</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const isSelf = u.id === currentUserId;
                return (
                  <tr key={u.id}>
                    <td className="primary-cell">{u.name}{isSelf ? " (tú)" : ""}</td>
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
          <p className="eyebrow">Nuevo miembro</p>
          <h2>Invitar usuario</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Nombre</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="field">
          <label>Email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </div>
        <div className="field">
          <label>Rol</label>
          <select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="tenant_agent">tenant_agent (acceso operativo)</option>
            <option value="tenant_admin">tenant_admin (gestiona equipo y settings)</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, justifyContent: "flex-start" }}>
        <button className="button" disabled={submitting}>
          {submitting ? "Creando…" : "Crear usuario"}
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
      toast.push("Configuración guardada", "success");
      onSaved();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="empty-state">Cargando configuración…</div>
      </section>
    );
  }
  if (error) {
    return (
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="empty-state danger">Error: {error}</div>
      </section>
    );
  }

  return (
    <form className="panel" style={{ marginTop: 16 }} onSubmit={handleSave}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">Operación de llamadas</p>
          <h2>Defaults del tenant</h2>
        </div>
        {dirty ? <span className="status warn">Cambios sin guardar</span> : <span className="status good">Sincronizado</span>}
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Zona horaria</label>
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
          <label>Caller ID por defecto</label>
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
          <label>Hora de inicio</label>
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
          <label>Hora de cierre</label>
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
          <label>Límite diario de llamadas</label>
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
          <label>Grabación de llamadas</label>
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={recordingEnabled}
              onChange={(e) => {
                setRecordingEnabled(e.target.checked);
                setDirty(true);
              }}
            />
            <span>
              Grabar todas las llamadas (requiere consentimiento del lead).
            </span>
          </label>
        </div>
        <div className="field" style={{ gridColumn: "1 / -1" }}>
          <label>Días permitidos</label>
          <div className="filter-row">
            {WEEKDAYS.map((d) => (
              <button
                type="button"
                key={d.key}
                className={`chip-button${allowedDays.includes(d.key) ? " active" : ""}`}
                onClick={() => toggleDay(d.key)}
              >
                {d.label}
              </button>
            ))}
          </div>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
        <button className="button" disabled={submitting || !dirty}>
          {submitting ? "Guardando…" : "Guardar cambios"}
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
          Descartar
        </button>
      </div>
      {settings ? (
        <p className="subtle" style={{ marginTop: 12 }}>
          Última actualización: {new Date(settings.updatedAt).toLocaleString()}
        </p>
      ) : null}
    </form>
  );
}

function VoiceCredentialsPanel({ tenant, canManage }: { tenant: string | undefined; canManage: boolean }) {
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
            <p className="eyebrow">Voz · proveedores</p>
            <h2>Credenciales</h2>
          </div>
        </div>
        <p className="subtle">Necesitas rol <code>tenant_admin</code> para gestionar las API keys de voz.</p>
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
      toast.push("Credenciales actualizadas", "success");
      setDraft({});
      creds.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
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
          <p className="eyebrow">Voz · proveedores</p>
          <h2>Credenciales del tenant</h2>
        </div>
        <div className="actions">
          {hasChanges ? <span className="status warn">Cambios sin guardar</span> : null}
          <button className="button" disabled={!hasChanges || submitting} onClick={handleSave}>
            <KeyRound aria-hidden="true" />
            <span>{submitting ? "Guardando…" : "Guardar"}</span>
          </button>
        </div>
      </div>

      <p className="subtle" style={{ marginBottom: 16 }}>
        Las claves se envían al voice-agent solo durante la creación de cada sesión y no se reflejan en el HTML.
        Si dejas un campo vacío, el voice-agent usa el valor por defecto del entorno. Lo que ves abajo es la
        clave enmascarada — reescribe encima para rotarla.
      </p>

      <div className="grid two">
        <ProviderBlock
          title="OpenAI Realtime"
          subtitle="ASR + LLM + TTS end-to-end. Recomendado para latencia mínima."
          c={c}
          draft={draft}
          fields={[
            { key: "openaiApiKey", label: "API key", type: "password", placeholder: "sk-..." },
            { key: "openaiRealtimeModel", label: "Modelo", placeholder: "gpt-4o-realtime-preview-2024-12-17" },
            { key: "openaiRealtimeVoice", label: "Voz", placeholder: "alloy" },
          ]}
          setField={setField}
        />
        <ProviderBlock
          title="Deepgram Voice Agent"
          subtitle="wss://agent.deepgram.com — listen + think + speak en un único socket."
          c={c}
          draft={draft}
          fields={[
            { key: "deepgramApiKey", label: "API key Deepgram", type: "password", placeholder: "..." },
            { key: "deepgramListenModel", label: "Modelo Listen (ASR)", placeholder: "nova-3" },
            { key: "deepgramThinkProvider", label: "Provider LLM", placeholder: "open_ai · anthropic · ..." },
            { key: "deepgramThinkModel", label: "Modelo LLM", placeholder: "gpt-4o-mini" },
            { key: "deepgramSpeakModel", label: "Modelo Speak (TTS)", placeholder: "aura-asteria-en" },
            { key: "deepgramGreeting", label: "Saludo inicial (opcional)", placeholder: "Hola, soy el asistente..." },
          ]}
          setField={setField}
        />
        <ProviderBlock
          title="AssemblyAI Voice Agent"
          subtitle="wss://agents.assemblyai.com — LLM y TTS hospedados por AssemblyAI."
          c={c}
          draft={draft}
          fields={[
            { key: "assemblyaiApiKey", label: "API key AssemblyAI", type: "password", placeholder: "..." },
            { key: "assemblyaiVoice", label: "Voz", placeholder: "ivy · james · tyler" },
            { key: "assemblyaiGreeting", label: "Saludo inicial (opcional)", placeholder: "Hola, soy el asistente..." },
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
  c,
  draft,
  fields,
  setField,
}: {
  title: string;
  subtitle: string;
  c: VoiceCredentials | null;
  draft: Partial<VoiceCredentials>;
  fields: { key: K; label: string; type?: string; placeholder?: string }[];
  setField: <KK extends keyof VoiceCredentials>(key: KK, value: VoiceCredentials[KK]) => void;
}) {
  return (
    <div className="panel" style={{ background: "var(--paper)", padding: 18 }}>
      <p className="eyebrow">{title}</p>
      <p className="subtle" style={{ marginBottom: 12 }}>{subtitle}</p>
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
