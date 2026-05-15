"use client";

// Editor de tools (function calling) para un bot.
//
// Se monta dentro del drawer del BotEditor cuando el bot ya está creado.
// Lista las tools existentes con su switch enabled, permite añadir
// nuevas con un formulario inline, editar y borrar.
//
// El JSON Schema de argumentos se edita como textarea — para MVP es
// suficiente; en una iteración futura podríamos hacer un builder visual.

import { useEffect, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useConfirm } from "./confirm";
import { useToast } from "./toast";
import {
  api,
  ApiError,
  BotTool,
  BotToolActionType,
  BotToolInput,
} from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";

const ACTION_TYPES: BotToolActionType[] = [
  "set_lead_outcome",
  "set_lead_status",
  "schedule_callback",
  "webhook",
  "end_call",
  "transfer_human",
  "search_knowledge_base",
];

type Props = {
  botId: string;
};

export function BotToolsEditor({ botId }: Props) {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const confirm = useConfirm();
  const [tools, setTools] = useState<BotTool[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<BotTool | null>(null);

  async function reload() {
    setLoading(true);
    try {
      const list = await api.botTools(botId, tenant);
      setTools(list);
    } catch {
      // si el bot no tiene tools aún, devolverá [] — no toast
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [botId, tenant]);

  function openCreate() {
    setEditing(null);
    setFormOpen(true);
  }
  function openEdit(tool: BotTool) {
    setEditing(tool);
    setFormOpen(true);
  }
  function closeForm() {
    setFormOpen(false);
    setEditing(null);
  }

  async function handleSave(input: BotToolInput) {
    try {
      if (editing) {
        await api.updateBotTool(botId, editing.id, input, tenant);
        toast.push(t("tools.toast.updated"), "success");
      } else {
        await api.createBotTool(botId, input, tenant);
        toast.push(t("tools.toast.created"), "success");
      }
      closeForm();
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  async function handleToggle(tool: BotTool) {
    try {
      await api.updateBotTool(
        botId,
        tool.id,
        {
          name: tool.name,
          description: tool.description,
          parametersSchema: tool.parametersSchema,
          actionType: tool.actionType,
          actionConfig: tool.actionConfig,
          enabled: !tool.enabled,
        },
        tenant
      );
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  async function handleDelete(tool: BotTool) {
    const ok = await confirm({
      title: t("tools.btn.delete"),
      description: t("tools.toast.delete_confirm", { name: tool.name }),
      variant: "danger",
      confirmLabel: t("tools.btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteBotTool(botId, tool.id, tenant);
      toast.push(t("tools.toast.deleted"), "success");
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  return (
    <section style={{ marginTop: 20, paddingTop: 20, borderTop: "1px solid var(--border)" }}>
      <div className="panel-header" style={{ marginBottom: 8 }}>
        <div>
          <p className="eyebrow">{t("tools.section.title")}</p>
          <p className="subtle" style={{ marginTop: 4, fontSize: 12.5 }}>
            {t("tools.section.desc")}
          </p>
        </div>
        {!formOpen ? (
          <button type="button" className="button secondary compact" onClick={openCreate}>
            <Plus aria-hidden="true" />
            <span>{t("tools.btn.add")}</span>
          </button>
        ) : null}
      </div>

      {formOpen ? (
        <ToolForm
          initial={editing ?? undefined}
          onSubmit={handleSave}
          onCancel={closeForm}
        />
      ) : null}

      {loading ? (
        <p className="subtle">{t("g.loading")}</p>
      ) : tools.length === 0 && !formOpen ? (
        <p className="subtle">{t("tools.empty")}</p>
      ) : (
        <div style={{ display: "grid", gap: 8 }}>
          {tools.map((tool) => (
            <div
              key={tool.id}
              className="command-row"
              style={{ alignItems: "flex-start", padding: "10px 12px" }}
            >
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  <strong className="mono" style={{ fontSize: 13 }}>{tool.name}</strong>
                  <span className="chip">{t(`tools.action.${tool.actionType}`)}</span>
                  <span className={`status ${tool.enabled ? "good" : ""}`}>
                    {tool.enabled ? t("tools.enabled") : t("tools.disabled")}
                  </span>
                </div>
                <p className="subtle" style={{ marginTop: 4, fontSize: 12.5 }}>
                  {tool.description}
                </p>
              </div>
              <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
                <label className="checkbox-row" style={{ marginRight: 6 }}>
                  <input
                    type="checkbox"
                    checked={tool.enabled}
                    onChange={() => handleToggle(tool)}
                  />
                </label>
                <button
                  type="button"
                  className="button ghost compact"
                  onClick={() => openEdit(tool)}
                  aria-label={t("tools.btn.edit")}
                >
                  <Pencil aria-hidden="true" />
                </button>
                <button
                  type="button"
                  className="button ghost compact"
                  onClick={() => handleDelete(tool)}
                  aria-label={t("tools.btn.delete")}
                >
                  <Trash2 aria-hidden="true" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function ToolForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial?: BotTool;
  onSubmit: (input: BotToolInput) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const toast = useToast();
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [actionType, setActionType] = useState<BotToolActionType>(
    initial?.actionType ?? "set_lead_outcome"
  );
  const [actionValue, setActionValue] = useState<string>(
    (initial?.actionConfig?.value as string) ?? ""
  );
  const [actionURL, setActionURL] = useState<string>(
    (initial?.actionConfig?.url as string) ?? ""
  );
  // Para search_knowledge_base partimos de un schema realista con query:string
  // requerido — sin eso el LLM no sabe qué argumento mandar.
  const defaultSchemaFor = (a: BotToolActionType): Record<string, unknown> => {
    if (a === "search_knowledge_base") {
      return {
        type: "object",
        properties: {
          query: {
            type: "string",
            description: "Natural-language question to retrieve from the knowledge base.",
          },
        },
        required: ["query"],
      };
    }
    return { type: "object", properties: {} };
  };
  const [paramsText, setParamsText] = useState(
    JSON.stringify(initial?.parametersSchema ?? defaultSchemaFor(initial?.actionType ?? "set_lead_outcome"), null, 2)
  );
  const [submitting, setSubmitting] = useState(false);
  const isEdit = Boolean(initial);

  // Cuando el usuario cambia action_type (en create), reseteamos el
  // schema al default del tipo si no lo ha editado. Heurística simple:
  // si el text actual matchea EXACTAMENTE el default del tipo anterior
  // lo consideramos "intacto" y lo sustituimos.
  function handleActionTypeChange(next: BotToolActionType) {
    const prevDefault = JSON.stringify(defaultSchemaFor(actionType), null, 2);
    if (paramsText === prevDefault) {
      setParamsText(JSON.stringify(defaultSchemaFor(next), null, 2));
    }
    setActionType(next);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    let parsed: Record<string, unknown> = {};
    try {
      parsed = JSON.parse(paramsText);
    } catch {
      toast.push(t("tools.toast.invalid_json"), "danger");
      return;
    }
    let config: Record<string, unknown> = {};
    if (actionType === "set_lead_outcome" || actionType === "set_lead_status") {
      config = { value: actionValue.trim() };
    } else if (actionType === "webhook") {
      config = { url: actionURL.trim() };
    }
    setSubmitting(true);
    try {
      await onSubmit({
        name: name.trim(),
        description: description.trim(),
        parametersSchema: parsed,
        actionType,
        actionConfig: config,
      });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="panel" onSubmit={submit} style={{ marginBottom: 12 }}>
      <div className="form-grid">
        <div className="field">
          <label>{t("tools.field.name")}</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("tools.field.name.placeholder")}
            required
            disabled={isEdit /* nombre se usa como id; cambiarlo confunde al LLM mid-flight */}
          />
          <p className="subtle" style={{ marginTop: 4, fontSize: 11.5 }}>
            {t("tools.field.name.hint")}
          </p>
        </div>
        <div className="field">
          <label>{t("tools.field.actiontype")}</label>
          <select
            value={actionType}
            onChange={(e) => handleActionTypeChange(e.target.value as BotToolActionType)}
            disabled={isEdit /* action_type es inmutable: ver backend */}
          >
            {ACTION_TYPES.map((a) => (
              <option key={a} value={a}>
                {t(`tools.action.${a}`)}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="field">
        <label>{t("tools.field.description")}</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder={t("tools.field.description.placeholder")}
          rows={2}
          required
        />
        <p className="subtle" style={{ marginTop: 4, fontSize: 11.5 }}>
          {t("tools.field.description.hint")}
        </p>
      </div>

      {actionType === "set_lead_outcome" || actionType === "set_lead_status" ? (
        <div className="field">
          <label>{t("tools.config.value")}</label>
          <input
            value={actionValue}
            onChange={(e) => setActionValue(e.target.value)}
            placeholder={
              actionType === "set_lead_outcome"
                ? t("tools.config.value.outcome.placeholder")
                : t("tools.config.value.status.placeholder")
            }
            required
          />
        </div>
      ) : null}
      {actionType === "webhook" ? (
        <div className="field">
          <label>{t("tools.config.url")}</label>
          <input
            value={actionURL}
            onChange={(e) => setActionURL(e.target.value)}
            placeholder={t("tools.config.url.placeholder")}
            type="url"
            required
          />
        </div>
      ) : null}
      {actionType === "search_knowledge_base" ? (
        <p className="subtle" style={{ fontSize: 12.5, marginTop: 4 }}>
          {t("tools.config.kb.hint")}
        </p>
      ) : null}

      <details style={{ marginTop: 10 }}>
        <summary className="subtle" style={{ cursor: "pointer", fontSize: 12.5 }}>
          {t("tools.field.params")}
        </summary>
        <textarea
          className="mono"
          value={paramsText}
          onChange={(e) => setParamsText(e.target.value)}
          rows={6}
          spellCheck={false}
          style={{ fontSize: 12, marginTop: 6 }}
        />
        <p className="subtle" style={{ marginTop: 4, fontSize: 11.5 }}>
          {t("tools.field.params.hint")}
        </p>
      </details>

      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          {t("tools.btn.cancel")}
        </button>
        <button className="button" disabled={submitting}>
          {submitting ? t("tools.btn.saving") : t("tools.btn.save")}
        </button>
      </div>
    </form>
  );
}
