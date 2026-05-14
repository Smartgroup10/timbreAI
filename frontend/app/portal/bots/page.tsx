"use client";

import { useEffect, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useToast } from "../../../components/toast";
import { api, ApiError, Bot, DID, VoiceCredentials, statusClass, statusLabel } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

const BOT_TYPES = ["renter_inbound", "owner_outbound", "support", "qualification"];
const BOT_STATUSES = ["draft", "active", "paused"];

type CatalogProvider = {
  id: string;
  label: string;
  models: { id: string; label: string }[];
  voices: { id: string; label: string }[];
  extraFields?: string[];
};

type EditState = { bot: Bot | null; mode: "edit" | "create" } | null;

export default function BotsPage() {
  const tenant = useTenantScope();
  const bots = useResource(() => api.bots(tenant), [tenant]);
  const dids = useResource(() => api.myDIDs(tenant), [tenant]);
  const toast = useToast();
  const [busyBot, setBusyBot] = useState<string | null>(null);
  const [editor, setEditor] = useState<EditState>(null);

  async function handleAssign(bot: Bot, didId: string) {
    setBusyBot(bot.id);
    try {
      await api.assignBotDID(bot.id, didId || null, tenant);
      toast.push(didId ? "DID asignado al bot" : "DID liberado", "success");
      bots.reload();
    } catch (err) {
      toast.push(`No se pudo asignar: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setBusyBot(null);
    }
  }

  async function handleDelete(bot: Bot) {
    if (!confirm(`Eliminar el bot "${bot.name}"? Las campañas que lo usan perderán su asignación.`)) return;
    try {
      await api.deleteBot(bot.id, tenant);
      toast.push("Bot eliminado", "success");
      bots.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  const availableDIDs = dids.data ?? [];

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Bots</h1>
          <p className="subtle">Identidad, objetivo, voz, guardrails y número saliente de cada asistente.</p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setEditor({ bot: null, mode: "create" })}>
            <Plus aria-hidden="true" />
            <span>Crear bot</span>
          </button>
        </div>
      </div>

      {availableDIDs.length === 0 ? (
        <div className="panel" style={{ marginBottom: 16 }}>
          <p className="eyebrow">Números disponibles</p>
          <h2>Aún no tienes DIDs asignados</h2>
          <p className="subtle">
            Pide al admin de plataforma que te asigne uno o más números desde <code>/admin/trunks</code>. Sin DID
            asignado, los bots solo pueden marcar a la extensión interna de sandbox.
          </p>
        </div>
      ) : null}

      {bots.loading ? <div className="empty-state">Cargando…</div> : null}
      {bots.error ? <div className="empty-state danger">Error: {bots.error}</div> : null}

      <div className="grid two">
        {(bots.data ?? []).map((bot) => (
          <BotCard
            key={bot.id}
            bot={bot}
            availableDIDs={availableDIDs}
            busy={busyBot === bot.id}
            onAssign={(didId) => handleAssign(bot, didId)}
            onEdit={() => setEditor({ bot, mode: "edit" })}
            onDelete={() => handleDelete(bot)}
          />
        ))}
      </div>

      {editor ? (
        <BotEditor
          bot={editor.bot}
          mode={editor.mode}
          onClose={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            bots.reload();
          }}
        />
      ) : null}
    </>
  );
}

function BotCard({
  bot,
  availableDIDs,
  busy,
  onAssign,
  onEdit,
  onDelete,
}: {
  bot: Bot;
  availableDIDs: DID[];
  busy: boolean;
  onAssign: (didId: string) => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">{bot.type}</p>
          <h2>{bot.name}</h2>
        </div>
        <span className={statusClass(bot.status)}>{statusLabel(bot.status)}</span>
      </div>
      <p className="subtle">{bot.objective}</p>
      <div className="filter-row">
        <span className="chip">Idioma: {bot.language}</span>
        <span className="chip">Voz: {bot.voice}</span>
        <span className="chip">Provider: {bot.voiceProvider || "echo"}</span>
      </div>

      <div className="field" style={{ marginTop: 8 }}>
        <label>Número saliente</label>
        <select
          value={bot.didId ?? ""}
          onChange={(e) => onAssign(e.target.value)}
          disabled={busy || availableDIDs.length === 0}
        >
          <option value="">— Sin DID (solo sandbox interno) —</option>
          {availableDIDs.map((did) => (
            <option key={did.id} value={did.id}>
              {did.e164}
              {did.label ? ` · ${did.label}` : ""}
            </option>
          ))}
        </select>
        {bot.didE164 ? (
          <p className="subtle" style={{ marginTop: 4 }}>
            Llamando como <code className="mono">{bot.didE164}</code>
          </p>
        ) : null}
      </div>

      <div className="command-strip" style={{ marginTop: 8 }}>
        {bot.guardrails.map((rule) => (
          <div className="command-row" key={rule}>
            <span>{rule}</span>
            <span className="status good">Regla</span>
          </div>
        ))}
      </div>

      <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
        <button className="button secondary compact" onClick={onEdit}>
          <Pencil aria-hidden="true" />
          <span>Editar</span>
        </button>
        <button className="button ghost compact" onClick={onDelete}>
          <Trash2 aria-hidden="true" />
          <span>Eliminar</span>
        </button>
      </div>
    </section>
  );
}

function BotEditor({
  bot,
  mode,
  onClose,
  onSaved,
}: {
  bot: Bot | null;
  mode: "edit" | "create";
  onClose: () => void;
  onSaved: () => void;
}) {
  const tenant = useTenantScope();
  const toast = useToast();
  const [name, setName] = useState(bot?.name ?? "");
  const [type, setType] = useState(bot?.type ?? "renter_inbound");
  const [language, setLanguage] = useState(bot?.language ?? "es-ES");
  const [voice, setVoice] = useState(bot?.voice ?? "");
  const [status, setStatus] = useState(bot?.status ?? "draft");
  const [objective, setObjective] = useState(bot?.objective ?? "");
  const [guardrails, setGuardrails] = useState((bot?.guardrails ?? []).join("\n"));
  const [voiceProvider, setVoiceProvider] = useState(bot?.voiceProvider ?? "echo");
  const [submitting, setSubmitting] = useState(false);

  // Catálogo estático de providers/voces (servido por backend) + qué providers
  // tienen API key en este tenant. Combinamos ambos para mostrar SOLO los
  // providers usables — el sandbox echo siempre.
  const [catalog, setCatalog] = useState<CatalogProvider[]>([]);
  const [enabledProviders, setEnabledProviders] = useState<Set<string>>(new Set(["echo"]));

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const [cat, creds] = await Promise.all([api.voiceCatalog(), api.voiceCredentials(tenant)]);
        if (cancelled) return;
        setCatalog(cat.providers);
        const enabled = new Set<string>(["echo"]);
        // El backend devuelve las API keys enmascaradas (•••• + últimos 4). Si
        // hay enmascarado, hay key real configurada → provider habilitado.
        if (creds.openaiApiKey) enabled.add("openai_realtime");
        if (creds.deepgramApiKey) enabled.add("deepgram");
        if (creds.assemblyaiApiKey) enabled.add("assemblyai");
        setEnabledProviders(enabled);
      } catch {
        // si falla, dejamos echo solamente — el operador sigue pudiendo crear bots.
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [tenant]);

  const currentProvider = catalog.find((p) => p.id === voiceProvider);
  const voiceOptions = currentProvider?.voices ?? [];

  // Si el voice actual no está en la lista del provider seleccionado, lo
  // limpiamos para evitar enviar basura al backend.
  useEffect(() => {
    if (voice && voiceOptions.length > 0 && !voiceOptions.some((v) => v.id === voice)) {
      setVoice(voiceOptions[0]?.id ?? "");
    }
  }, [voiceProvider, voiceOptions.length]); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!name.trim()) {
      toast.push("Nombre requerido", "warn");
      return;
    }
    const guardrailsArray = guardrails
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    setSubmitting(true);
    try {
      const payload = { name, type, language, voice, status, objective, voiceProvider, guardrails: guardrailsArray };
      if (mode === "create") {
        await api.createBot(payload, tenant);
      } else if (bot) {
        await api.updateBot(bot.id, payload, tenant);
      }
      toast.push(mode === "create" ? "Bot creado" : "Bot actualizado", "success");
      onSaved();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label="Cerrar" />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{mode === "create" ? "Nuevo bot" : "Editar bot"}</p>
            <h2>{mode === "create" ? "Crear asistente" : bot?.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            Cerrar
          </button>
        </header>

        <form className="drawer-body" onSubmit={handleSubmit}>
          <div className="field">
            <label>Nombre</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="form-grid">
            <div className="field">
              <label>Tipo</label>
              <select value={type} onChange={(e) => setType(e.target.value)}>
                {BOT_TYPES.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label>Estado</label>
              <select value={status} onChange={(e) => setStatus(e.target.value)}>
                {BOT_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label>Idioma</label>
              <input value={language} onChange={(e) => setLanguage(e.target.value)} placeholder="es-ES" />
            </div>
            <div className="field">
              <label>Voz</label>
              <select value={voice} onChange={(e) => setVoice(e.target.value)} disabled={voiceOptions.length === 0}>
                {voiceOptions.length === 0 ? (
                  <option value="">— No aplica para este provider —</option>
                ) : (
                  <>
                    <option value="">— Default del provider —</option>
                    {voiceOptions.map((v) => (
                      <option key={v.id} value={v.id}>
                        {v.label}
                      </option>
                    ))}
                  </>
                )}
              </select>
            </div>
          </div>
          <div className="field">
            <label>Provider de voz</label>
            <select value={voiceProvider} onChange={(e) => setVoiceProvider(e.target.value)}>
              <option value="echo">Echo (sandbox sin API key)</option>
              {catalog.map((p) => {
                const isEnabled = enabledProviders.has(p.id);
                return (
                  <option key={p.id} value={p.id} disabled={!isEnabled}>
                    {p.label}
                    {isEnabled ? "" : " — falta API key en Configuración"}
                  </option>
                );
              })}
            </select>
            <p className="subtle" style={{ marginTop: 4 }}>
              {voiceProvider === "echo"
                ? "Devuelve el audio que recibe. Útil para pruebas sin gasto en LLM/TTS."
                : enabledProviders.has(voiceProvider)
                ? `Llamadas con este bot usarán las credenciales del tenant para ${currentProvider?.label}.`
                : `Configura primero la API key en /portal/settings antes de usar ${currentProvider?.label}.`}
            </p>
          </div>
          <div className="field">
            <label>Objetivo</label>
            <textarea
              value={objective}
              onChange={(e) => setObjective(e.target.value)}
              placeholder="Qué tiene que conseguir este bot al final de la llamada."
            />
          </div>
          <div className="field">
            <label>Guardrails (una regla por línea)</label>
            <textarea
              value={guardrails}
              onChange={(e) => setGuardrails(e.target.value)}
              placeholder={"Identificarse como asistente IA\nNo inventar precios"}
            />
          </div>
          <button className="button" disabled={submitting}>
            {submitting ? "Guardando…" : mode === "create" ? "Crear bot" : "Guardar cambios"}
          </button>
        </form>
      </aside>
    </div>
  );
}
