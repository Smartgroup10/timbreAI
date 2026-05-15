"use client";

// Panel de reglas de routing por DID. Va dentro de la página
// /admin/trunks (tab DIDs). El operador platform_admin selecciona
// un DID y se le muestra el listado de reglas + form inline para
// crear/editar.
//
// Las reglas se evalúan en el backend por priority ASC; la primera
// que matchea decide qué bot atiende. Si ninguna matchea, fallback
// al bot default del DID (bots.did_id).

import { useEffect, useMemo, useState } from "react";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { useConfirm } from "./confirm";
import { useToast } from "./toast";
import {
  api,
  ApiError,
  Bot,
  DID,
  DIDRoutingRule,
  DIDRoutingRuleInput,
} from "../lib/api";
import { useT } from "../lib/i18n";

export function DIDRoutingPanel({ did, onClose }: { did: DID; onClose: () => void }) {
  const t = useT();
  const toast = useToast();
  const confirm = useConfirm();

  const [rules, setRules] = useState<DIDRoutingRule[]>([]);
  const [bots, setBots] = useState<Bot[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<DIDRoutingRule | null>(null);

  const tenantId = did.tenantId ?? "";

  async function reload() {
    setLoading(true);
    try {
      const [rs, bs] = await Promise.all([
        api.adminDIDRoutingRules(did.id),
        tenantId ? api.bots(tenantId) : Promise.resolve([] as Bot[]),
      ]);
      setRules(rs);
      setBots(bs);
    } catch {
      // ignorar — el panel sigue funcional vacío
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [did.id, tenantId]);

  async function handleSave(input: DIDRoutingRuleInput) {
    try {
      if (editing) {
        await api.adminUpdateDIDRoutingRule(did.id, editing.id, input);
        toast.push(t("admin.trunks.routing.toast.updated"), "success");
      } else {
        await api.adminCreateDIDRoutingRule(did.id, input);
        toast.push(t("admin.trunks.routing.toast.created"), "success");
      }
      setFormOpen(false);
      setEditing(null);
      await reload();
    } catch (err) {
      toast.push(
        t("admin.trunks.routing.toast.save_failed", { err: err instanceof ApiError ? err.code : "error" }),
        "danger",
      );
    }
  }

  async function handleDelete(rule: DIDRoutingRule) {
    const ok = await confirm({
      title: t("admin.trunks.routing.btn.delete"),
      description: t("admin.trunks.routing.toast.delete_confirm"),
      variant: "danger",
      confirmLabel: t("admin.trunks.routing.btn.delete"),
    });
    if (!ok) return;
    try {
      await api.adminDeleteDIDRoutingRule(did.id, rule.id);
      toast.push(t("admin.trunks.routing.toast.deleted"), "success");
      await reload();
    } catch (err) {
      toast.push(
        t("admin.trunks.routing.toast.delete_failed", { err: err instanceof ApiError ? err.code : "error" }),
        "danger",
      );
    }
  }

  if (!tenantId) {
    return (
      <div className="panel" style={{ marginBottom: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{did.e164}</p>
            <h2>{t("admin.trunks.routing.title")}</h2>
          </div>
          <button className="button ghost" onClick={onClose}>
            {t("admin.trunks.routing.form.cancel")}
          </button>
        </div>
        <p className="subtle">{t("admin.trunks.routing.notenant")}</p>
      </div>
    );
  }

  return (
    <div className="panel" style={{ marginBottom: 16 }}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">{did.e164}{did.label ? ` · ${did.label}` : ""}</p>
          <h2>{t("admin.trunks.routing.title")}</h2>
          <p className="subtle" style={{ marginTop: 4 }}>{t("admin.trunks.routing.subtitle")}</p>
        </div>
        <div className="actions" style={{ gap: 8 }}>
          {bots.length > 0 && !formOpen ? (
            <button
              className="button"
              onClick={() => {
                setEditing(null);
                setFormOpen(true);
              }}
            >
              <Plus size={14} style={{ marginRight: 4 }} />
              {t("admin.trunks.routing.btn.new")}
            </button>
          ) : null}
          <button className="button ghost" onClick={onClose}>
            {t("admin.trunks.routing.form.cancel")}
          </button>
        </div>
      </div>

      {bots.length === 0 ? (
        <p className="subtle">{t("admin.trunks.routing.nobots")}</p>
      ) : null}

      {formOpen ? (
        <RoutingRuleForm
          initial={editing ?? undefined}
          bots={bots}
          onSubmit={handleSave}
          onCancel={() => {
            setFormOpen(false);
            setEditing(null);
          }}
        />
      ) : null}

      {loading ? (
        <p className="subtle">…</p>
      ) : rules.length === 0 && !formOpen ? (
        <p className="subtle">{t("admin.trunks.routing.empty")}</p>
      ) : rules.length > 0 ? (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("admin.trunks.routing.col.priority")}</th>
                <th>{t("admin.trunks.routing.col.name")}</th>
                <th>{t("admin.trunks.routing.col.schedule")}</th>
                <th>{t("admin.trunks.routing.col.days")}</th>
                <th>{t("admin.trunks.routing.col.caller")}</th>
                <th>{t("admin.trunks.routing.col.lang")}</th>
                <th>{t("admin.trunks.routing.col.bot")}</th>
                <th>{t("admin.trunks.routing.col.enabled")}</th>
                <th>{t("admin.trunks.routing.col.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <RuleRow
                  key={r.id}
                  rule={r}
                  bots={bots}
                  onEdit={() => {
                    setEditing(r);
                    setFormOpen(true);
                  }}
                  onDelete={() => handleDelete(r)}
                />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}

function RuleRow({
  rule,
  bots,
  onEdit,
  onDelete,
}: {
  rule: DIDRoutingRule;
  bots: Bot[];
  onEdit: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  const targetBotName = rule.targetBotName || bots.find((b) => b.id === rule.targetBotId)?.name || rule.targetBotId;
  return (
    <tr>
      <td>{rule.priority}</td>
      <td className="primary-cell">{rule.name}</td>
      <td>
        {rule.startMinute == null || rule.endMinute == null
          ? t("admin.trunks.routing.always")
          : `${fmtMinute(rule.startMinute)} – ${fmtMinute(rule.endMinute)}`}
      </td>
      <td>{fmtDays(rule.daysOfWeek, t)}</td>
      <td>
        {rule.callerPrefixes.length === 0
          ? t("admin.trunks.routing.allcallers")
          : rule.callerPrefixes.join(", ")}
      </td>
      <td>{rule.language ? rule.language : t("admin.trunks.routing.alllangs")}</td>
      <td>
        <span className="chip">{targetBotName}</span>
      </td>
      <td>
        <span className={rule.enabled ? "status good" : "status"}>
          {rule.enabled ? "✓" : "—"}
        </span>
      </td>
      <td>
        <button className="button ghost compact" onClick={onEdit} aria-label="edit">
          <Pencil size={14} />
        </button>
        <button className="button ghost compact" style={{ marginLeft: 6 }} onClick={onDelete} aria-label="delete">
          <Trash2 size={14} />
        </button>
      </td>
    </tr>
  );
}

function RoutingRuleForm({
  initial,
  bots,
  onSubmit,
  onCancel,
}: {
  initial?: DIDRoutingRule;
  bots: Bot[];
  onSubmit: (input: DIDRoutingRuleInput) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const editing = Boolean(initial);
  const [name, setName] = useState(initial?.name ?? "");
  const [priority, setPriority] = useState(initial?.priority ?? 100);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [timezone, setTimezone] = useState(initial?.timezone ?? "Europe/Madrid");
  const [daysOfWeek, setDaysOfWeek] = useState<number[]>(initial?.daysOfWeek ?? []);
  const [startTime, setStartTime] = useState(initial?.startMinute != null ? fmtMinute(initial.startMinute) : "");
  const [endTime, setEndTime] = useState(initial?.endMinute != null ? fmtMinute(initial.endMinute) : "");
  const [prefixes, setPrefixes] = useState((initial?.callerPrefixes ?? []).join(", "));
  const [language, setLanguage] = useState(initial?.language ?? "");
  const [targetBotId, setTargetBotId] = useState(initial?.targetBotId ?? bots[0]?.id ?? "");
  const [fallbackBotId, setFallbackBotId] = useState(initial?.fallbackBotId ?? "");
  const [submitting, setSubmitting] = useState(false);

  function toggleDay(d: number) {
    setDaysOfWeek((cur) => (cur.includes(d) ? cur.filter((x) => x !== d) : [...cur, d].sort()));
  }

  const days = useMemo(() => [0, 1, 2, 3, 4, 5, 6], []);

  return (
    <form
      className="panel"
      style={{ marginBottom: 12, background: "var(--surface-2, #f8f9fb)" }}
      onSubmit={async (e) => {
        e.preventDefault();
        setSubmitting(true);
        const start = parseTime(startTime);
        const end = parseTime(endTime);
        const callerPrefixes = prefixes
          .split(",")
          .map((s) => s.trim())
          .filter((s) => s.length > 0);
        await onSubmit({
          name: name.trim(),
          priority,
          enabled,
          timezone: timezone.trim() || "Europe/Madrid",
          daysOfWeek,
          startMinute: start,
          endMinute: end,
          callerPrefixes,
          language: language.trim(),
          targetBotId,
          fallbackBotId: fallbackBotId || null,
        });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <h3 style={{ margin: 0 }}>
            {editing ? t("admin.trunks.routing.form.edit.title") : t("admin.trunks.routing.form.new.title")}
          </h3>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("admin.trunks.routing.form.name")}</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            placeholder={t("admin.trunks.routing.form.name.placeholder")}
          />
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.priority")}</label>
          <input
            type="number"
            value={priority}
            onChange={(e) => setPriority(parseInt(e.target.value, 10) || 100)}
          />
          <p className="subtle" style={{ fontSize: 12, marginTop: 4 }}>
            {t("admin.trunks.routing.form.priority.hint")}
          </p>
        </div>
        <div className="field">
          <label>
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              style={{ marginRight: 6 }}
            />
            {t("admin.trunks.routing.form.enabled")}
          </label>
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.timezone")}</label>
          <input value={timezone} onChange={(e) => setTimezone(e.target.value)} placeholder="Europe/Madrid" />
        </div>
        <div className="field" style={{ gridColumn: "1 / -1" }}>
          <label>{t("admin.trunks.routing.form.days")}</label>
          <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
            {days.map((d) => (
              <button
                type="button"
                key={d}
                onClick={() => toggleDay(d)}
                className={`chip-button${daysOfWeek.includes(d) ? " active" : ""}`}
              >
                {t(`admin.trunks.routing.dow.${d}`)}
              </button>
            ))}
          </div>
          <p className="subtle" style={{ fontSize: 12, marginTop: 4 }}>
            {t("admin.trunks.routing.form.days.hint")}
          </p>
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.start")}</label>
          <input type="time" value={startTime} onChange={(e) => setStartTime(e.target.value)} />
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.end")}</label>
          <input type="time" value={endTime} onChange={(e) => setEndTime(e.target.value)} />
        </div>
        <div className="field" style={{ gridColumn: "1 / -1" }}>
          <p className="subtle" style={{ fontSize: 12, margin: 0 }}>
            {t("admin.trunks.routing.form.hours.hint")}
          </p>
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.prefixes")}</label>
          <input
            value={prefixes}
            onChange={(e) => setPrefixes(e.target.value)}
            placeholder={t("admin.trunks.routing.form.prefixes.placeholder")}
          />
          <p className="subtle" style={{ fontSize: 12, marginTop: 4 }}>
            {t("admin.trunks.routing.form.prefixes.hint")}
          </p>
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.language")}</label>
          <input value={language} onChange={(e) => setLanguage(e.target.value)} placeholder="es" />
          <p className="subtle" style={{ fontSize: 12, marginTop: 4 }}>
            {t("admin.trunks.routing.form.language.hint")}
          </p>
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.target")}</label>
          <select value={targetBotId} onChange={(e) => setTargetBotId(e.target.value)} required>
            {bots.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>{t("admin.trunks.routing.form.fallback")}</label>
          <select value={fallbackBotId ?? ""} onChange={(e) => setFallbackBotId(e.target.value)}>
            <option value="">{t("admin.trunks.routing.form.fallback.none")}</option>
            {bots.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          {t("admin.trunks.routing.form.cancel")}
        </button>
        <button className="button" disabled={submitting || !targetBotId || !name.trim()}>
          {submitting
            ? "…"
            : editing
            ? t("admin.trunks.routing.form.submit.edit")
            : t("admin.trunks.routing.form.submit.create")}
        </button>
      </div>
    </form>
  );
}

// Helpers de formato hora/minuto/día.

function fmtMinute(m: number): string {
  const h = Math.floor(m / 60);
  const mm = m % 60;
  return `${String(h).padStart(2, "0")}:${String(mm).padStart(2, "0")}`;
}

function parseTime(s: string): number | null {
  if (!s) return null;
  const [hh, mm] = s.split(":");
  const h = parseInt(hh, 10);
  const m = parseInt(mm, 10);
  if (isNaN(h) || isNaN(m)) return null;
  return h * 60 + m;
}

function fmtDays(days: number[], t: ReturnType<typeof useT>): string {
  if (!days || days.length === 0) return t("admin.trunks.routing.alldays");
  const sorted = [...days].sort();
  return sorted.map((d) => t(`admin.trunks.routing.dow.${d}`)).join(", ");
}
