"use client";

import { useEffect, useState } from "react";
import { Bot as BotIcon, Pencil, Plus, Trash2 } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { CardGridSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import { api, ApiError, Bot, DID, statusClass } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../lib/i18n";

const BOT_TYPES = ["renter_inbound", "owner_outbound", "support", "qualification"];
const BOT_STATUSES = ["draft", "active", "paused"];

// Lista de idiomas/locales soportados por los providers de voz. El primero
// de cada familia (es-ES, en-US) es el default cuando detectamos idioma
// genérico desde el ID de voz (-es / -en).
const LANGUAGE_OPTIONS = [
  "es-ES",
  "es-MX",
  "es-AR",
  "es-CO",
  "en-US",
  "en-GB",
  "pt-PT",
  "pt-BR",
  "fr-FR",
  "de-DE",
  "it-IT",
];

// detectLangFromVoice intenta inferir el locale desde el id o label de una
// voz. Devuelve null si no puede — entonces dejamos el idioma como esté
// (p. ej. voces OpenAI Realtime que son multilingüe).
//
// Reglas:
//   - id acaba en "-es" / "-en" / "-pt" → mapea al locale primario de la familia
//   - label contiene "(ES" / "(EN" / "(PT" / etc. → idem
//   - el resto: null (no tocamos el idioma)
function detectLangFromVoice(voiceId: string, voiceLabel: string): string | null {
  const id = voiceId.toLowerCase();
  const label = voiceLabel.toUpperCase();

  // 1. Sufijo en el id (patrón Deepgram aura-2-celeste-es).
  if (/-es$/.test(id) || /-es-/.test(id)) return "es-ES";
  if (/-en$/.test(id) || /-en-/.test(id)) return "en-US";
  if (/-pt$/.test(id) || /-pt-/.test(id)) return "pt-PT";
  if (/-fr$/.test(id) || /-fr-/.test(id)) return "fr-FR";
  if (/-de$/.test(id) || /-de-/.test(id)) return "de-DE";
  if (/-it$/.test(id) || /-it-/.test(id)) return "it-IT";

  // 2. Marcador en el label "(ES, ...)" etc.
  if (/\(ES[,)\s]/.test(label)) return "es-ES";
  if (/\(EN[,)\s]/.test(label)) return "en-US";
  if (/\(PT[,)\s]/.test(label)) return "pt-PT";
  if (/\(FR[,)\s]/.test(label)) return "fr-FR";
  if (/\(DE[,)\s]/.test(label)) return "de-DE";
  if (/\(IT[,)\s]/.test(label)) return "it-IT";

  return null;
}

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
  const t = useT();
  const statusLabel = useStatusLabel();
  const bots = useResource(() => api.bots(tenant), [tenant]);
  const dids = useResource(() => api.myDIDs(tenant), [tenant]);
  const toast = useToast();
  const confirm = useConfirm();
  const [busyBot, setBusyBot] = useState<string | null>(null);
  const [editor, setEditor] = useState<EditState>(null);

  async function handleAssign(bot: Bot, didId: string) {
    setBusyBot(bot.id);
    try {
      await api.assignBotDID(bot.id, didId || null, tenant);
      toast.push(didId ? t("bots.toast.assigned") : t("bots.toast.released"), "success");
      bots.reload();
    } catch (err) {
      toast.push(t("bots.toast.assign_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setBusyBot(null);
    }
  }

  async function handleDelete(bot: Bot) {
    const ok = await confirm({
      title: t("btn.delete"),
      description: t("bots.toast.delete_confirm", { name: bot.name }),
      variant: "danger",
      confirmLabel: t("btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteBot(bot.id, tenant);
      toast.push(t("bots.toast.deleted"), "success");
      bots.reload();
    } catch (err) {
      toast.push(t("bots.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  const availableDIDs = dids.data ?? [];

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("bots.title")}</h1>
          <p className="subtle">{t("bots.subtitle.full")}</p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setEditor({ bot: null, mode: "create" })}>
            <Plus aria-hidden="true" />
            <span>{t("bots.btn.create")}</span>
          </button>
        </div>
      </div>

      {availableDIDs.length === 0 ? (
        <div className="panel" style={{ marginBottom: 16 }}>
          <p className="eyebrow">{t("bots.didempty.eyebrow")}</p>
          <h2>{t("bots.didempty.title")}</h2>
          <p className="subtle">{t("bots.didempty.body", { path: "/admin/trunks" })}</p>
        </div>
      ) : null}

      {bots.loading ? <CardGridSkeleton count={2} /> : null}
      {bots.error ? <div className="empty-state danger">{t("g.error")}: {bots.error}</div> : null}

      {!bots.loading && !bots.error && (bots.data?.length ?? 0) === 0 ? (
        <EmptyState
          icon={BotIcon}
          title={t("bots.empty")}
          description={t("bots.empty.desc")}
          action={{ label: t("bots.btn.create"), onClick: () => setEditor({ bot: null, mode: "create" }) }}
        />
      ) : null}

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
  const t = useT();
  const statusLabel = useStatusLabel();
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
        <span className="chip">{t("bots.card.language", { v: bot.language })}</span>
        <span className="chip">{t("bots.card.voice", { v: bot.voice })}</span>
        <span className="chip">{t("bots.card.provider", { v: bot.voiceProvider || "echo" })}</span>
      </div>

      <div className="field" style={{ marginTop: 8 }}>
        <label>{t("bots.card.did")}</label>
        <select
          value={bot.didId ?? ""}
          onChange={(e) => onAssign(e.target.value)}
          disabled={busy || availableDIDs.length === 0}
        >
          <option value="">{t("bots.card.did.none")}</option>
          {availableDIDs.map((did) => (
            <option key={did.id} value={did.id}>
              {did.e164}
              {did.label ? ` · ${did.label}` : ""}
            </option>
          ))}
        </select>
        {bot.didE164 ? (
          <p className="subtle" style={{ marginTop: 4 }}>
            {t("bots.card.callingas")} <code className="mono">{bot.didE164}</code>
          </p>
        ) : null}
      </div>

      <div className="command-strip" style={{ marginTop: 8 }}>
        {bot.guardrails.map((rule) => (
          <div className="command-row" key={rule}>
            <span>{rule}</span>
            <span className="status good">{t("bots.card.rule")}</span>
          </div>
        ))}
      </div>

      <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
        <button className="button secondary compact" onClick={onEdit}>
          <Pencil aria-hidden="true" />
          <span>{t("bots.card.edit")}</span>
        </button>
        <button className="button ghost compact" onClick={onDelete}>
          <Trash2 aria-hidden="true" />
          <span>{t("bots.card.delete")}</span>
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
  const t = useT();
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
        if (creds.openaiApiKey) enabled.add("openai_realtime");
        if (creds.deepgramApiKey) enabled.add("deepgram");
        if (creds.assemblyaiApiKey) enabled.add("assemblyai");
        setEnabledProviders(enabled);
      } catch {
        // ignore — fallback al sandbox echo
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [tenant]);

  const currentProvider = catalog.find((p) => p.id === voiceProvider);
  const voiceOptions = currentProvider?.voices ?? [];

  useEffect(() => {
    if (voice && voiceOptions.length > 0 && !voiceOptions.some((v) => v.id === voice)) {
      setVoice(voiceOptions[0]?.id ?? "");
    }
  }, [voiceProvider, voiceOptions.length]); // eslint-disable-line react-hooks/exhaustive-deps

  // Cuando cambia la voz, intentamos auto-actualizar el idioma — pero solo
  // si la FAMILIA cambia (es → en, no es-MX → es-ES). Así respetamos la
  // variante regional que el usuario haya elegido a mano.
  function handleVoiceChange(newVoiceId: string) {
    setVoice(newVoiceId);
    const opt = voiceOptions.find((v) => v.id === newVoiceId);
    if (!opt) return;
    const detected = detectLangFromVoice(opt.id, opt.label);
    if (!detected) return;
    const currentFamily = language.split("-")[0];
    const detectedFamily = detected.split("-")[0];
    if (currentFamily !== detectedFamily) {
      setLanguage(detected);
    }
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!name.trim()) {
      toast.push(t("bots.editor.name.required"), "warn");
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
      toast.push(mode === "create" ? t("bots.editor.toast.created") : t("bots.editor.toast.updated"), "success");
      onSaved();
    } catch (err) {
      toast.push(t("bots.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label={t("btn.close")} />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{mode === "create" ? t("bots.editor.create.eyebrow") : t("bots.editor.edit.eyebrow")}</p>
            <h2>{mode === "create" ? t("bots.editor.create.title") : bot?.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            {t("btn.close")}
          </button>
        </header>

        <form className="drawer-body" onSubmit={handleSubmit}>
          <div className="field">
            <label>{t("col.name")}</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="form-grid">
            <div className="field">
              <label>{t("bots.editor.field.type")}</label>
              <select value={type} onChange={(e) => setType(e.target.value)}>
                {BOT_TYPES.map((tp) => (
                  <option key={tp} value={tp}>
                    {tp}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label>{t("bots.editor.field.status")}</label>
              <select value={status} onChange={(e) => setStatus(e.target.value)}>
                {BOT_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label>{t("bots.editor.field.language")}</label>
              <select value={language} onChange={(e) => setLanguage(e.target.value)}>
                {/* Si el bot venía con un locale fuera de la lista, lo
                    preservamos como primera opción para no sobreescribirlo
                    silenciosamente al guardar. */}
                {language && !LANGUAGE_OPTIONS.includes(language) ? (
                  <option value={language}>{language}</option>
                ) : null}
                {LANGUAGE_OPTIONS.map((loc) => (
                  <option key={loc} value={loc}>
                    {t(`bots.editor.lang.${loc}`)}
                  </option>
                ))}
              </select>
              <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>
                {t("bots.editor.lang.autohint")}
              </p>
            </div>
            <div className="field">
              <label>{t("bots.editor.field.voice")}</label>
              <select value={voice} onChange={(e) => handleVoiceChange(e.target.value)} disabled={voiceOptions.length === 0}>
                {voiceOptions.length === 0 ? (
                  <option value="">{t("bots.editor.voice.none")}</option>
                ) : (
                  <>
                    <option value="">{t("bots.editor.voice.default")}</option>
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
            <label>{t("bots.editor.field.provider")}</label>
            <select value={voiceProvider} onChange={(e) => setVoiceProvider(e.target.value)}>
              <option value="echo">{t("bots.editor.provider.echo")}</option>
              {catalog.map((p) => {
                const isEnabled = enabledProviders.has(p.id);
                return (
                  <option key={p.id} value={p.id} disabled={!isEnabled}>
                    {p.label}
                    {isEnabled ? "" : t("bots.editor.provider.missingkey")}
                  </option>
                );
              })}
            </select>
            <p className="subtle" style={{ marginTop: 4 }}>
              {voiceProvider === "echo"
                ? t("bots.editor.provider.echo.hint")
                : enabledProviders.has(voiceProvider)
                ? t("bots.editor.provider.enabled.hint", { provider: currentProvider?.label ?? voiceProvider })
                : t("bots.editor.provider.disabled.hint", { provider: currentProvider?.label ?? voiceProvider })}
            </p>
          </div>
          <div className="field">
            <label>{t("bots.editor.field.objective")}</label>
            <textarea
              value={objective}
              onChange={(e) => setObjective(e.target.value)}
              placeholder={t("bots.editor.objective.placeholder")}
            />
          </div>
          <div className="field">
            <label>{t("bots.editor.field.guardrails")}</label>
            <textarea
              value={guardrails}
              onChange={(e) => setGuardrails(e.target.value)}
              placeholder={t("bots.editor.guardrails.placeholder")}
            />
          </div>
          <button className="button" disabled={submitting}>
            {submitting ? t("bots.editor.submit.creating") : mode === "create" ? t("bots.editor.submit.create") : t("bots.editor.submit.save")}
          </button>
        </form>
      </aside>
    </div>
  );
}
