"use client";

import { useEffect, useState } from "react";
import { Megaphone, Pause, Play, Trash2, Users } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { CardGridSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import { api, ApiError, Bot, Call, Campaign, CampaignLead, Lead, statusClass } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../lib/i18n";

// Convierte un timestamp ISO del backend a lo que espera <input type="datetime-local">
// (formato "YYYY-MM-DDTHH:mm" en zona local). Devuelve "" si la fecha es null.
function toLocalInput(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// Convierte lo que el navegador devuelve en datetime-local (zona local) a ISO 8601
// con offset. Empty = null (para "clear").
function fromLocalInput(v: string): string | null {
  if (!v) return null;
  const d = new Date(v);
  if (isNaN(d.getTime())) return null;
  return d.toISOString();
}

export default function CampaignsPage() {
  const tenant = useTenantScope();
  const t = useT();
  const statusLabel = useStatusLabel();
  const campaigns = useResource(() => api.campaigns(tenant), [tenant], { pollMs: 15_000 });
  const bots = useResource(() => api.bots(tenant), [tenant]);
  const [formOpen, setFormOpen] = useState(false);
  const [leadsDrawer, setLeadsDrawer] = useState<Campaign | null>(null);
  const toast = useToast();
  const confirm = useConfirm();

  async function handleCreate(input: Partial<Campaign>) {
    try {
      await api.createCampaign(input, tenant);
      toast.push(t("campaigns.toast.created"), "success");
      setFormOpen(false);
      campaigns.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("campaigns.toast.create_failed", { err: code }), "danger");
    }
  }

  async function handleLaunch(c: Campaign) {
    try {
      await api.updateCampaign(c.id, { status: "active" }, tenant);
      toast.push(t("campaigns.toast.launched"), "success");
      campaigns.reload();
    } catch (err) {
      toast.push(t("campaigns.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handlePause(c: Campaign) {
    try {
      await api.updateCampaign(c.id, { status: "paused" }, tenant);
      toast.push(t("campaigns.toast.paused"), "success");
      campaigns.reload();
    } catch (err) {
      toast.push(t("campaigns.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleDelete(c: Campaign) {
    const ok = await confirm({
      title: t("btn.delete"),
      description: t("campaigns.toast.delete_confirm", { name: c.name }),
      variant: "danger",
      confirmLabel: t("btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteCampaign(c.id, tenant);
      toast.push(t("campaigns.toast.deleted"), "success");
      campaigns.reload();
    } catch (err) {
      toast.push(t("campaigns.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("campaigns.title")}</h1>
          <p className="subtle">{t("campaigns.subtitle.full")}</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setFormOpen((v) => !v)}>
            {formOpen ? t("campaigns.btn.cancel") : t("campaigns.new")}
          </button>
        </div>
      </div>

      {formOpen ? <CampaignForm bots={bots.data ?? []} onSubmit={handleCreate} /> : null}

      {campaigns.loading ? <CardGridSkeleton count={2} /> : null}

      {!campaigns.loading && (campaigns.data?.length ?? 0) === 0 && !formOpen ? (
        <EmptyState
          icon={Megaphone}
          title={t("campaigns.empty")}
          description={t("campaigns.empty.desc")}
          action={{ label: t("campaigns.new"), onClick: () => setFormOpen(true) }}
        />
      ) : null}

      <div className="grid two" style={{ marginBottom: 16 }}>
        {(campaigns.data ?? []).map((campaign) => (
          <section className="panel" key={campaign.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">{t("campaigns.drawer.eyebrow")}</p>
                <h2>{campaign.name}</h2>
              </div>
              <span className={statusClass(campaign.status)}>{statusLabel(campaign.status)}</span>
            </div>
            <div className="command-strip">
              <div className="command-row">
                <span>{t("campaigns.start")}</span>
                <strong>
                  {campaign.startAt ? new Date(campaign.startAt).toLocaleString() : t("campaigns.start.immediate")}
                </strong>
              </div>
              <div className="command-row">
                <span>{t("campaigns.end")}</span>
                <strong>
                  {campaign.endAt ? new Date(campaign.endAt).toLocaleString() : t("campaigns.end.unlimited")}
                </strong>
              </div>
              <div className="command-row">
                <span>{t("col.bot")}</span>
                <strong>{(bots.data ?? []).find((b) => b.id === campaign.botId)?.name || campaign.botId || "—"}</strong>
              </div>
              <div className="command-row">
                <span>{t("col.leads")}</span>
                <strong>{campaign.leadCount}</strong>
              </div>
              <div className="command-row">
                <span>{t("col.concurrency")}</span>
                <strong>{t("campaigns.parallel", { n: campaign.maxConcurrent })}</strong>
              </div>
              <div className="command-row">
                <span>{t("col.retries")}</span>
                <strong>{campaign.maxAttempts}</strong>
              </div>
            </div>
            <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
              <button className="button compact" onClick={() => setLeadsDrawer(campaign)}>
                <Users aria-hidden="true" />
                <span>{t("campaigns.btn.manageleads")}</span>
              </button>
              {campaign.status === "active" ? (
                <button className="button secondary compact" onClick={() => handlePause(campaign)}>
                  <Pause aria-hidden="true" />
                  <span>{t("campaigns.btn.pause")}</span>
                </button>
              ) : (
                <button className="button secondary compact" onClick={() => handleLaunch(campaign)}>
                  <Play aria-hidden="true" />
                  <span>{t("campaigns.btn.launch")}</span>
                </button>
              )}
              <button className="button ghost compact" onClick={() => handleDelete(campaign)}>
                <Trash2 aria-hidden="true" />
                <span>{t("campaigns.btn.delete")}</span>
              </button>
            </div>
          </section>
        ))}
      </div>

      {leadsDrawer ? (
        <CampaignLeadsDrawer
          campaign={leadsDrawer}
          tenant={tenant}
          onClose={() => setLeadsDrawer(null)}
          onChanged={() => campaigns.reload()}
        />
      ) : null}

      <section className="panel">
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("campaigns.control.eyebrow")}</p>
            <h2>{t("campaigns.control.title")}</h2>
          </div>
        </div>
        <div className="grid three">
          <div>
            <h3>{t("campaigns.control.consent.title")}</h3>
            <p className="subtle">{t("campaigns.control.consent.desc")}</p>
          </div>
          <div>
            <h3>{t("campaigns.control.schedule.title")}</h3>
            <p className="subtle">{t("campaigns.control.schedule.desc")}</p>
          </div>
          <div>
            <h3>{t("campaigns.control.volume.title")}</h3>
            <p className="subtle">{t("campaigns.control.volume.desc")}</p>
          </div>
        </div>
      </section>
    </>
  );
}

function CampaignForm({ bots, onSubmit }: { bots: Bot[]; onSubmit: (input: Partial<Campaign>) => Promise<void> }) {
  const t = useT();
  const [name, setName] = useState("");
  const [botId, setBotId] = useState(bots[0]?.id ?? "");
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const [maxConcurrent, setMaxConcurrent] = useState(3);
  const [maxAttempts, setMaxAttempts] = useState(2);
  const [status, setStatus] = useState("draft");
  const [submitting, setSubmitting] = useState(false);

  return (
    <form
      className="panel"
      style={{ marginBottom: 16 }}
      onSubmit={async (event) => {
        event.preventDefault();
        setSubmitting(true);
        await onSubmit({
          name,
          botId,
          status,
          maxAttempts,
          maxConcurrent,
          startAt: fromLocalInput(startAt),
          endAt: fromLocalInput(endAt),
          schedule: startAt ? `${startAt} → ${endAt || "∞"}` : "",
        });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">{t("campaigns.form.eyebrow")}</p>
          <h2>{t("campaigns.form.title")}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("campaigns.field.name")}</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder={t("campaigns.form.name.placeholder")} />
        </div>
        <div className="field">
          <label>{t("col.bot")}</label>
          <select value={botId} onChange={(e) => setBotId(e.target.value)} required>
            <option value="">{t("campaigns.form.bot.placeholder")}</option>
            {bots.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>{t("campaigns.form.startat")}</label>
          <input type="datetime-local" value={startAt} onChange={(e) => setStartAt(e.target.value)} />
          <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>{t("campaigns.form.startat.hint")}</p>
        </div>
        <div className="field">
          <label>{t("campaigns.form.endat")}</label>
          <input type="datetime-local" value={endAt} onChange={(e) => setEndAt(e.target.value)} />
          <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>{t("campaigns.form.endat.hint")}</p>
        </div>
        <div className="field">
          <label>{t("campaigns.form.concurrency")}</label>
          <input
            type="number"
            min={1}
            max={50}
            value={maxConcurrent}
            onChange={(e) => setMaxConcurrent(parseInt(e.target.value, 10) || 1)}
          />
          <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>{t("campaigns.form.concurrency.hint")}</p>
        </div>
        <div className="field">
          <label>{t("campaigns.form.attempts")}</label>
          <input
            type="number"
            min={1}
            max={10}
            value={maxAttempts}
            onChange={(e) => setMaxAttempts(parseInt(e.target.value, 10) || 1)}
          />
        </div>
        <div className="field">
          <label>{t("campaigns.form.status")}</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="draft">{t("campaigns.form.status.draft")}</option>
            <option value="active">{t("campaigns.form.status.active")}</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? t("campaigns.form.submitting") : t("campaigns.form.submit")}
        </button>
      </div>
    </form>
  );
}

type DrawerTab = "leads" | "add" | "calls";

function CampaignLeadsDrawer({
  campaign,
  tenant,
  onClose,
  onChanged,
}: {
  campaign: Campaign;
  tenant: string | undefined;
  onClose: () => void;
  onChanged: () => void;
}) {
  const t = useT();
  const statusLabel = useStatusLabel();
  const [tab, setTab] = useState<DrawerTab>("leads");
  const [leads, setLeads] = useState<CampaignLead[]>([]);
  const [available, setAvailable] = useState<Lead[]>([]);
  const [calls, setCalls] = useState<Call[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const toast = useToast();

  async function reload() {
    setLoading(true);
    try {
      const [campLeads, allLeads, allCalls] = await Promise.all([
        api.campaignLeads(campaign.id, tenant),
        api.leads(tenant),
        api.calls(tenant),
      ]);
      setLeads(campLeads);
      const inCampaign = new Set(campLeads.map((cl) => cl.leadId));
      setAvailable(allLeads.filter((l) => !inCampaign.has(l.id)));
      setCalls(allCalls.filter((c) => c.campaignId === campaign.id));
    } catch (err) {
      toast.push(t("campaigns.drawer.toast.loaderror", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [campaign.id]);

  // Refresco automático cada 10s cuando la pestaña "Llamadas" está abierta —
  // así el operador ve en vivo qué leads están siendo marcados ahora mismo
  // sin recargar la página.
  useEffect(() => {
    if (tab !== "calls") return;
    const id = setInterval(reload, 10_000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, campaign.id]);

  async function handleAdd() {
    if (selected.size === 0) return;
    setAdding(true);
    try {
      const r = await api.addCampaignLeads(campaign.id, Array.from(selected), tenant);
      toast.push(t("campaigns.drawer.toast.added", { n: r.created }), "success");
      setSelected(new Set());
      await reload();
      onChanged();
      setTab("leads");
    } catch (err) {
      toast.push(t("campaigns.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setAdding(false);
    }
  }

  async function handleRemove(leadId: string) {
    try {
      await api.removeCampaignLead(campaign.id, leadId, tenant);
      toast.push(t("campaigns.drawer.toast.removed"), "success");
      await reload();
      onChanged();
    } catch (err) {
      toast.push(t("campaigns.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label={t("btn.close")} />
      <aside className="drawer wide">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{t("campaigns.drawer.eyebrow")}</p>
            <h2>{campaign.name}</h2>
            <p className="subtle" style={{ marginTop: 4 }}>
              {t("campaigns.drawer.summary", { leads: leads.length, calls: calls.length, n: campaign.maxConcurrent })}
            </p>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            {t("btn.close")}
          </button>
        </header>

        <div className="filter-row" style={{ padding: "12px 24px 0", margin: 0 }}>
          <button
            className={`chip-button${tab === "leads" ? " active" : ""}`}
            onClick={() => setTab("leads")}
          >
            {t("campaigns.drawer.tab.leads.count", { n: leads.length })}
          </button>
          <button
            className={`chip-button${tab === "calls" ? " active" : ""}`}
            onClick={() => setTab("calls")}
          >
            {t("campaigns.drawer.tab.calls.count", { n: calls.length })}
          </button>
          <button
            className={`chip-button${tab === "add" ? " active" : ""}`}
            onClick={() => setTab("add")}
          >
            {t("campaigns.drawer.tab.add.count", { n: available.length })}
          </button>
        </div>

        <div className="drawer-body">
          {tab === "leads" ? (
            loading ? (
              <p className="subtle">{t("g.loading")}</p>
            ) : leads.length === 0 ? (
              <p className="subtle">{t("campaigns.drawer.empty.leads")}</p>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th>{t("col.name")}</th>
                    <th>{t("col.phone")}</th>
                    <th>{t("col.status")}</th>
                    <th>{t("col.attempts")}</th>
                    <th>{t("col.lastattempt")}</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {leads.map((cl) => (
                    <tr key={cl.id}>
                      <td className="primary-cell">{cl.leadName || "—"}</td>
                      <td>
                        <code className="mono">{cl.leadPhone || ""}</code>
                      </td>
                      <td>
                        <span className={statusClass(cl.status)}>{statusLabel(cl.status)}</span>
                      </td>
                      <td>{cl.attempts}</td>
                      <td className="subtle">
                        {cl.lastAttemptAt ? new Date(cl.lastAttemptAt).toLocaleString() : "—"}
                      </td>
                      <td>
                        <button className="button ghost compact" onClick={() => handleRemove(cl.leadId)}>
                          {t("campaigns.drawer.remove")}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )
          ) : tab === "calls" ? (
            loading ? (
              <p className="subtle">{t("g.loading")}</p>
            ) : calls.length === 0 ? (
              <p className="subtle">{t("campaigns.drawer.empty.calls")}</p>
            ) : (
              <>
                <p className="subtle" style={{ marginBottom: 12 }}>{t("campaigns.drawer.calls.hint")}</p>
                <table>
                  <thead>
                    <tr>
                      <th>{t("col.lead")}</th>
                      <th>{t("col.phone")}</th>
                      <th>{t("col.status")}</th>
                      <th>{t("col.outcome")}</th>
                      <th>{t("col.duration")}</th>
                      <th>{t("col.start")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {calls.map((c) => (
                      <tr key={c.id}>
                        <td>{c.leadName || "—"}</td>
                        <td>
                          <code className="mono">{c.phone}</code>
                        </td>
                        <td>
                          <span className={statusClass(c.status)}>{statusLabel(c.status)}</span>
                        </td>
                        <td>
                          <span className={statusClass(c.outcome)}>{statusLabel(c.outcome)}</span>
                        </td>
                        <td>{c.durationSec > 0 ? `${c.durationSec}s` : "—"}</td>
                        <td className="subtle">
                          {c.startedAt ? new Date(c.startedAt).toLocaleString() : statusLabel("queued")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )
          ) : (
            available.length === 0 ? (
              <p className="subtle">
                {t("campaigns.drawer.empty.add")} {t("campaigns.drawer.empty.add.cta")}{" "}
                <a href="/portal/leads">{t("nav.leads")}</a>.
              </p>
            ) : (
              <>
                <div style={{ maxHeight: 380, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 8 }}>
                  <table>
                    <thead>
                      <tr>
                        <th style={{ width: 40 }}></th>
                        <th>{t("col.name")}</th>
                        <th>{t("col.phone")}</th>
                        <th>{t("col.type")}</th>
                        <th>{t("col.status")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {available.map((l) => (
                        <tr key={l.id}>
                          <td>
                            <input
                              type="checkbox"
                              checked={selected.has(l.id)}
                              onChange={(e) => {
                                const next = new Set(selected);
                                if (e.target.checked) next.add(l.id);
                                else next.delete(l.id);
                                setSelected(next);
                              }}
                            />
                          </td>
                          <td>{l.name}</td>
                          <td>
                            <code className="mono">{l.phone}</code>
                          </td>
                          <td>{l.type}</td>
                          <td>
                            <span className={statusClass(l.status)}>{statusLabel(l.status)}</span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="actions" style={{ marginTop: 12 }}>
                  <button className="button" disabled={adding || selected.size === 0} onClick={handleAdd}>
                    {t("campaigns.drawer.add.button")} {selected.size > 0 ? `(${selected.size})` : ""}
                  </button>
                </div>
              </>
            )
          )}
        </div>
      </aside>
    </div>
  );
}
