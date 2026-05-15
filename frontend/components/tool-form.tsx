"use client";

// ToolForm — formulario de crear/editar una tool de la biblioteca.
// Lo usa la página /portal/tools. El bot editor pasa a usar
// BotToolAssignments (otro componente) que solo selecciona qué tools
// asignar de las que YA existen en la biblioteca.

import { useState } from "react";
import { useToast } from "./toast";
import { BotToolActionType, Tool, ToolInput } from "../lib/api";
import { useT } from "../lib/i18n";

const ACTION_TYPES: BotToolActionType[] = [
  "set_lead_outcome",
  "set_lead_status",
  "schedule_callback",
  "webhook",
  "end_call",
  "transfer_human",
  "search_knowledge_base",
  "calendar_check_availability",
  "calendar_schedule_meeting",
  "calendar_list_my_meetings",
  "calendar_cancel_meeting",
  "calendar_reschedule_meeting",
];

// JSON Schema sugerido por tipo. Si el operador no lo edita, viaja
// pre-rellenado al backend; si lo edita, respetamos lo que ponga.
function defaultSchemaFor(a: BotToolActionType): Record<string, unknown> {
  switch (a) {
    case "search_knowledge_base":
      return {
        type: "object",
        properties: {
          query: { type: "string", description: "Natural-language question to retrieve from the knowledge base." },
        },
        required: ["query"],
      };
    case "calendar_check_availability":
      return {
        type: "object",
        properties: {
          date: { type: "string", description: "Date to check in YYYY-MM-DD format. Defaults to today." },
          duration_min: { type: "integer", description: "Minimum slot length in minutes. Defaults to 30." },
          timezone: { type: "string", description: "IANA timezone (e.g. Europe/Madrid)." },
        },
      };
    case "calendar_schedule_meeting":
      return {
        type: "object",
        properties: {
          start_time: { type: "string", description: "Meeting start in RFC3339." },
          duration_min: { type: "integer", description: "Duration in minutes. Defaults to 30." },
          title: { type: "string", description: "Event title shown in calendar." },
          description: { type: "string", description: "Optional notes about the meeting." },
          attendee_email: { type: "string", description: "Lead email if available — Google sends them an invite." },
        },
        required: ["start_time"],
      };
    case "calendar_list_my_meetings":
      return {
        type: "object",
        properties: {
          timezone: { type: "string", description: "IANA timezone to format times. Defaults to Europe/Madrid." },
        },
      };
    case "calendar_cancel_meeting":
      return {
        type: "object",
        properties: {
          meeting_id: { type: "string", description: "Reference returned by list_my_meetings. Must belong to the caller." },
        },
        required: ["meeting_id"],
      };
    case "calendar_reschedule_meeting":
      return {
        type: "object",
        properties: {
          meeting_id: { type: "string", description: "Reference returned by list_my_meetings. Must belong to the caller." },
          new_start_time: { type: "string", description: "New start in RFC3339." },
          duration_min: { type: "integer", description: "New duration in minutes. Keeps original if omitted." },
        },
        required: ["meeting_id", "new_start_time"],
      };
    default:
      return { type: "object", properties: {} };
  }
}

export function ToolForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial?: Tool;
  onSubmit: (input: ToolInput) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const toast = useToast();
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [actionType, setActionType] = useState<BotToolActionType>(initial?.actionType ?? "set_lead_outcome");
  const [actionValue, setActionValue] = useState<string>((initial?.actionConfig?.value as string) ?? "");
  const [actionURL, setActionURL] = useState<string>((initial?.actionConfig?.url as string) ?? "");
  const [paramsText, setParamsText] = useState(
    JSON.stringify(initial?.parametersSchema ?? defaultSchemaFor(initial?.actionType ?? "set_lead_outcome"), null, 2)
  );
  const [submitting, setSubmitting] = useState(false);
  const isEdit = Boolean(initial);

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
            disabled={isEdit /* nombre como identificador; cambiarlo confunde al LLM mid-flight */}
          />
          <p className="subtle" style={{ marginTop: 4, fontSize: 11.5 }}>{t("tools.field.name.hint")}</p>
        </div>
        <div className="field">
          <label>{t("tools.field.actiontype")}</label>
          <select
            value={actionType}
            onChange={(e) => handleActionTypeChange(e.target.value as BotToolActionType)}
            disabled={isEdit /* action_type inmutable backend-side */}
          >
            {ACTION_TYPES.map((a) => (
              <option key={a} value={a}>{t(`tools.action.${a}`)}</option>
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
        <p className="subtle" style={{ marginTop: 4, fontSize: 11.5 }}>{t("tools.field.description.hint")}</p>
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
        <p className="subtle" style={{ fontSize: 12.5, marginTop: 4 }}>{t("tools.config.kb.hint")}</p>
      ) : null}
      {actionType === "calendar_check_availability" || actionType === "calendar_schedule_meeting" ? (
        <p className="subtle" style={{ fontSize: 12.5, marginTop: 4 }}>{t("tools.config.cal.hint")}</p>
      ) : null}
      {actionType === "calendar_list_my_meetings" ||
      actionType === "calendar_cancel_meeting" ||
      actionType === "calendar_reschedule_meeting" ? (
        <p className="subtle" style={{ fontSize: 12.5, marginTop: 4 }}>{t("tools.config.cal.manage.hint")}</p>
      ) : null}

      <details style={{ marginTop: 10 }}>
        <summary className="subtle" style={{ cursor: "pointer", fontSize: 12.5 }}>{t("tools.field.params")}</summary>
        <textarea
          className="mono"
          value={paramsText}
          onChange={(e) => setParamsText(e.target.value)}
          rows={6}
          spellCheck={false}
          style={{ fontSize: 12, marginTop: 6 }}
        />
        <p className="subtle" style={{ marginTop: 4, fontSize: 11.5 }}>{t("tools.field.params.hint")}</p>
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
