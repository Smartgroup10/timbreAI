"use client";

// Panel de webhooks salientes para integrar con CRMs externos.
// Va dentro de la página /portal/settings.
//
// Flujo:
//   - Lista webhooks existentes con su estado activo/desactivado.
//   - Crear: form con name/url/events. La respuesta del POST incluye
//     `secret` UNA SOLA VEZ — lo mostramos en un modal y advertimos.
//   - Editar: name/url/events (el secret no se toca aquí; rotar es un
//     acción aparte).
//   - Rotar secret: confirma + POST /regenerate → muestra el nuevo secret.
//   - Eliminar: confirm + DELETE.
//   - Histórico: tabla con últimas 50 entregas (cualquier endpoint).

import { useEffect, useState } from "react";
import { ExternalLink, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useConfirm } from "./confirm";
import { useToast } from "./toast";
import {
  api,
  ApiError,
  WebhookDelivery,
  WebhookEndpoint,
  WebhookEndpointInput,
} from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";

export function WebhooksPanel() {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const confirm = useConfirm();
  const [webhooks, setWebhooks] = useState<WebhookEndpoint[]>([]);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [availableEvents, setAvailableEvents] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<WebhookEndpoint | null>(null);
  // Secret recién generado para mostrar en modal. null = sin modal.
  const [newSecret, setNewSecret] = useState<{ name: string; secret: string } | null>(null);

  async function reload() {
    setLoading(true);
    try {
      const [list, dlist, ev] = await Promise.all([
        api.webhooks(tenant),
        api.webhookDeliveries(tenant),
        api.webhookEvents(),
      ]);
      setWebhooks(list);
      setDeliveries(dlist);
      setAvailableEvents(ev.events);
    } catch {
      // sin tools aún → []
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant]);

  async function handleSave(input: WebhookEndpointInput) {
    try {
      if (editing) {
        await api.updateWebhook(editing.id, input, tenant);
        toast.push(t("webhooks.toast.updated"), "success");
      } else {
        const created = await api.createWebhook(input, tenant);
        toast.push(t("webhooks.toast.created"), "success");
        if (created.secret) {
          setNewSecret({ name: created.name, secret: created.secret });
        }
      }
      setFormOpen(false);
      setEditing(null);
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("webhooks.toast.error", { err: code }), "danger");
    }
  }

  async function handleDelete(wh: WebhookEndpoint) {
    const ok = await confirm({
      title: t("webhooks.btn.delete"),
      description: t("webhooks.btn.delete.confirm", { name: wh.name }),
      variant: "danger",
      confirmLabel: t("webhooks.btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteWebhook(wh.id, tenant);
      toast.push(t("webhooks.toast.deleted"), "success");
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("webhooks.toast.error", { err: code }), "danger");
    }
  }

  async function handleRotate(wh: WebhookEndpoint) {
    const ok = await confirm({
      title: t("webhooks.btn.regenerate"),
      description: t("webhooks.btn.regenerate.confirm", { name: wh.name }),
      variant: "danger",
      confirmLabel: t("webhooks.btn.regenerate"),
    });
    if (!ok) return;
    try {
      const r = await api.regenerateWebhookSecret(wh.id, tenant);
      toast.push(t("webhooks.toast.regenerated"), "success");
      setNewSecret({ name: wh.name, secret: r.secret });
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("webhooks.toast.error", { err: code }), "danger");
    }
  }

  async function handleToggle(wh: WebhookEndpoint) {
    try {
      await api.updateWebhook(
        wh.id,
        { name: wh.name, url: wh.url, events: wh.events, active: !wh.active },
        tenant
      );
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("webhooks.toast.error", { err: code }), "danger");
    }
  }

  return (
    <>
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("webhooks.eyebrow")}</p>
            <h2>{t("webhooks.title")}</h2>
            <p className="subtle" style={{ marginTop: 4 }}>{t("webhooks.desc")}</p>
          </div>
          {!formOpen ? (
            <button
              type="button"
              className="button compact"
              onClick={() => {
                setEditing(null);
                setFormOpen(true);
              }}
            >
              <Plus aria-hidden="true" />
              <span>{t("webhooks.btn.new")}</span>
            </button>
          ) : null}
        </div>

        {formOpen ? (
          <WebhookForm
            initial={editing ?? undefined}
            availableEvents={availableEvents}
            onCancel={() => {
              setFormOpen(false);
              setEditing(null);
            }}
            onSubmit={handleSave}
          />
        ) : null}

        {loading ? (
          <p className="subtle">{t("g.loading")}</p>
        ) : webhooks.length === 0 && !formOpen ? (
          <p className="subtle">{t("webhooks.empty")}</p>
        ) : (
          <div style={{ display: "grid", gap: 10, marginTop: 14 }}>
            {webhooks.map((wh) => (
              <div key={wh.id} className="command-row" style={{ alignItems: "flex-start" }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <strong>{wh.name}</strong>
                    <span className={`status ${wh.active ? "good" : ""}`}>
                      {wh.active ? t("tools.enabled") : t("tools.disabled")}
                    </span>
                  </div>
                  <p className="subtle mono" style={{ marginTop: 4, fontSize: 12, wordBreak: "break-all" }}>
                    <a href={wh.url} target="_blank" rel="noreferrer" style={{ color: "inherit" }}>
                      {wh.url} <ExternalLink aria-hidden="true" style={{ width: 11, height: 11, verticalAlign: "middle" }} />
                    </a>
                  </p>
                  <div style={{ display: "flex", gap: 4, flexWrap: "wrap", marginTop: 6 }}>
                    {wh.events.map((e) => (
                      <span key={e} className="chip">
                        {t(`webhooks.event.${e}`)}
                      </span>
                    ))}
                  </div>
                </div>
                <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
                  <label className="checkbox-row" style={{ marginRight: 6 }}>
                    <input type="checkbox" checked={wh.active} onChange={() => handleToggle(wh)} />
                  </label>
                  <button
                    type="button"
                    className="button ghost compact"
                    onClick={() => handleRotate(wh)}
                    aria-label={t("webhooks.btn.regenerate")}
                    title={t("webhooks.btn.regenerate")}
                  >
                    <RefreshCw aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    className="button ghost compact"
                    onClick={() => {
                      setEditing(wh);
                      setFormOpen(true);
                    }}
                    aria-label={t("webhooks.btn.edit")}
                  >
                    <Pencil aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    className="button ghost compact"
                    onClick={() => handleDelete(wh)}
                    aria-label={t("webhooks.btn.delete")}
                  >
                    <Trash2 aria-hidden="true" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("webhooks.eyebrow")}</p>
            <h2>{t("webhooks.deliveries.title")}</h2>
          </div>
        </div>
        {deliveries.length === 0 ? (
          <p className="subtle">{t("webhooks.deliveries.empty")}</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t("webhooks.deliveries.col.when")}</th>
                  <th>{t("webhooks.deliveries.col.event")}</th>
                  <th>{t("webhooks.deliveries.col.status")}</th>
                  <th>{t("webhooks.deliveries.col.attempt")}</th>
                  <th>{t("webhooks.deliveries.col.error")}</th>
                </tr>
              </thead>
              <tbody>
                {deliveries.map((d) => {
                  const ok = d.statusCode >= 200 && d.statusCode < 300;
                  return (
                    <tr key={d.id}>
                      <td>
                        <time>{new Date(d.createdAt).toLocaleString()}</time>
                      </td>
                      <td>
                        <span className="chip">{t(`webhooks.event.${d.eventType}`)}</span>
                      </td>
                      <td>
                        <span className={ok ? "status good" : "status danger"}>
                          {d.statusCode > 0 ? d.statusCode : t("webhooks.deliveries.col.error")}
                        </span>
                      </td>
                      <td>{d.attempt}</td>
                      <td className="summary-cell subtle">{d.error || (ok ? t("webhooks.deliveries.ok") : "—")}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {newSecret ? (
        <SecretReveal
          name={newSecret.name}
          secret={newSecret.secret}
          onClose={() => setNewSecret(null)}
        />
      ) : null}
    </>
  );
}

function WebhookForm({
  initial,
  availableEvents,
  onSubmit,
  onCancel,
}: {
  initial?: WebhookEndpoint;
  availableEvents: string[];
  onSubmit: (input: WebhookEndpointInput) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const [name, setName] = useState(initial?.name ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [events, setEvents] = useState<string[]>(initial?.events ?? []);
  const [active, setActive] = useState(initial?.active ?? true);
  const [submitting, setSubmitting] = useState(false);

  function toggleEvent(ev: string) {
    setEvents((prev) => (prev.includes(ev) ? prev.filter((e) => e !== ev) : [...prev, ev]));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await onSubmit({ name: name.trim(), url: url.trim(), events, active });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="panel" onSubmit={submit} style={{ marginBottom: 12, marginTop: 14 }}>
      <div className="form-grid">
        <div className="field">
          <label>{t("webhooks.field.name")}</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("webhooks.field.name.placeholder")}
            required
          />
        </div>
        <div className="field">
          <label>{t("webhooks.field.url")}</label>
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t("webhooks.field.url.placeholder")}
            required
          />
        </div>
      </div>
      <div className="field" style={{ marginTop: 8 }}>
        <label>{t("webhooks.field.events")}</label>
        <div className="filter-row" style={{ marginTop: 4 }}>
          {availableEvents.map((ev) => (
            <button
              type="button"
              key={ev}
              className={`chip-button${events.includes(ev) ? " active" : ""}`}
              onClick={() => toggleEvent(ev)}
            >
              {t(`webhooks.event.${ev}`)}
            </button>
          ))}
        </div>
      </div>
      <div className="field" style={{ marginTop: 8 }}>
        <label className="checkbox-row">
          <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
          <span>{t("webhooks.field.active")}</span>
        </label>
      </div>
      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          {t("webhooks.btn.cancel")}
        </button>
        <button className="button" disabled={submitting || events.length === 0}>
          {submitting ? t("webhooks.btn.saving") : t("webhooks.btn.save")}
        </button>
      </div>
    </form>
  );
}

function SecretReveal({
  name,
  secret,
  onClose,
}: {
  name: string;
  secret: string;
  onClose: () => void;
}) {
  const t = useT();
  const toast = useToast();
  function copy() {
    navigator.clipboard?.writeText(secret).then(() => {
      toast.push(t("webhooks.secret.copied"), "success");
    });
  }
  return (
    <div className="confirm-overlay" role="presentation">
      <button type="button" className="confirm-backdrop" aria-label={t("webhooks.secret.close")} onClick={onClose} />
      <div className="confirm-dialog" role="alertdialog" aria-modal="true">
        <h2 className="confirm-title">{t("webhooks.secret.title")} — {name}</h2>
        <p className="confirm-desc">{t("webhooks.secret.desc")}</p>
        <pre className="code-block" style={{ marginTop: 12, wordBreak: "break-all", whiteSpace: "pre-wrap" }}>
          {secret}
        </pre>
        <div className="confirm-actions">
          <button type="button" className="button secondary" onClick={copy}>
            {t("webhooks.secret.copy")}
          </button>
          <button type="button" className="button" onClick={onClose}>
            {t("webhooks.secret.close")}
          </button>
        </div>
      </div>
    </div>
  );
}
