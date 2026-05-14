"use client";

import { use, useState } from "react";
import Link from "next/link";
import { ArrowLeft, PhoneCall, Trash2 } from "lucide-react";
import { useConfirm } from "../../../../components/confirm";
import { TestCallDrawer } from "../../../../components/test-call-drawer";
import { useToast } from "../../../../components/toast";
import { api, ApiError, statusClass } from "../../../../lib/api";
import { useTenantScope } from "../../../../lib/auth-context";
import { useResource } from "../../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../../lib/i18n";

const STATUS_OPTIONS = ["new", "qualified", "callback", "contacted", "do_not_call"];

export default function LeadDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const tenant = useTenantScope();
  const t = useT();
  const statusLabel = useStatusLabel();
  const lead = useResource(() => api.getLead(id, tenant), [id, tenant]);
  const calls = useResource(() => api.leadCalls(id, tenant), [id, tenant], { pollMs: 15_000 });
  const toast = useToast();
  const confirm = useConfirm();
  const [drawerOpen, setDrawerOpen] = useState(false);

  async function handleStatus(status: string) {
    try {
      await api.updateLead(id, { status }, tenant);
      toast.push(t("leads.toast.status_updated"), "success");
      lead.reload();
      calls.reload();
    } catch (err) {
      toast.push(t("leads.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleDelete() {
    if (!lead.data) return;
    const ok = await confirm({
      title: t("btn.delete"),
      description: t("leads.detail.delete.confirm", { name: lead.data.name }),
      variant: "danger",
      confirmLabel: t("btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteLead(id, tenant);
      toast.push(t("leads.toast.deleted"), "success");
      window.location.href = "/portal/leads";
    } catch (err) {
      toast.push(t("leads.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  if (lead.loading) return <div className="empty-state">{t("leads.detail.loading")}</div>;
  if (lead.error) return <div className="empty-state danger">{t("g.error")}: {lead.error}</div>;
  if (!lead.data) return <div className="empty-state">{t("leads.detail.notfound")}</div>;

  const l = lead.data;
  const callList = calls.data ?? [];
  const totalDuration = callList.reduce((acc, c) => acc + c.durationSec, 0);

  function formatDuration(sec: number) {
    if (!sec) return "—";
    if (sec < 60) return `${sec}s`;
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return s === 0 ? `${m}m` : `${m}m ${s}s`;
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <Link href="/portal/leads" className="button ghost compact" style={{ marginBottom: 8 }}>
            <ArrowLeft aria-hidden="true" />
            <span>{t("leads.detail.back")}</span>
          </Link>
          <p className="eyebrow">{t("leads.detail.eyebrow")}</p>
          <h1>{l.name}</h1>
          <p className="subtle">
            <code className="mono">{l.phone}</code> · {l.email || t("leads.detail.noemail")}
          </p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setDrawerOpen(true)}>
            <PhoneCall aria-hidden="true" />
            <span>{t("leads.detail.testcall")}</span>
          </button>
          <button className="button ghost" onClick={handleDelete}>
            <Trash2 aria-hidden="true" />
            <span>{t("leads.detail.delete")}</span>
          </button>
        </div>
      </div>

      <div className="grid two">
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("leads.detail.data.eyebrow")}</p>
              <h2>{t("leads.detail.data.title")}</h2>
            </div>
            <span className={statusClass(l.status)}>{statusLabel(l.status)}</span>
          </div>
          <div className="command-strip">
            <Row label={t("leads.detail.type")} value={<span className="chip">{l.type}</span>} />
            <Row label={t("leads.detail.source")} value={l.source} />
            <Row label={t("leads.detail.consent")} value={l.consent} />
            <Row label={t("leads.detail.lastactivity")} value={new Date(l.lastActivity).toLocaleString()} />
          </div>
          <div className="field" style={{ marginTop: 14 }}>
            <label>{t("leads.detail.changestatus")}</label>
            <select value={l.status} onChange={(e) => handleStatus(e.target.value)}>
              {STATUS_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("leads.detail.activity.eyebrow")}</p>
              <h2>{t("leads.detail.activity.title")}</h2>
            </div>
          </div>
          <div className="command-strip">
            <Row label={t("leads.detail.totalcalls")} value={<strong>{callList.length}</strong>} />
            <Row label={t("leads.detail.totaltime")} value={formatDuration(totalDuration)} />
            <Row
              label={t("leads.detail.lastcall")}
              value={
                callList[0]?.startedAt
                  ? new Date(callList[0].startedAt).toLocaleString()
                  : <span className="subtle">{t("leads.detail.lastcall.never")}</span>
              }
            />
            <Row
              label={t("leads.detail.topoutcome")}
              value={
                callList.length === 0 ? (
                  <span className="subtle">—</span>
                ) : (
                  <span className="chip">
                    {statusLabel(topOutcome(callList.map((c) => c.outcome)))}
                  </span>
                )
              }
            />
          </div>
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("leads.detail.history.eyebrow")}</p>
            <h2>{t("leads.detail.history.title", { n: callList.length })}</h2>
          </div>
        </div>
        {calls.loading ? (
          <div className="empty-state">{t("g.loading")}</div>
        ) : callList.length === 0 ? (
          <div className="empty-state">{t("leads.detail.history.empty")}</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t("leads.detail.col.date")}</th>
                  <th>{t("col.campaign")}</th>
                  <th>{t("col.status")}</th>
                  <th>{t("col.outcome")}</th>
                  <th>{t("col.duration")}</th>
                  <th>{t("col.summary")}</th>
                </tr>
              </thead>
              <tbody>
                {callList.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <Link href={`/portal/calls/${c.id}`} style={{ color: "inherit" }}>
                        {c.startedAt ? new Date(c.startedAt).toLocaleString() : "—"}
                      </Link>
                    </td>
                    <td>{c.campaign || t("leads.detail.manual")}</td>
                    <td>
                      <span className={statusClass(c.status)}>{statusLabel(c.status)}</span>
                    </td>
                    <td>
                      <span className="chip">{statusLabel(c.outcome)}</span>
                    </td>
                    <td>{formatDuration(c.durationSec)}</td>
                    <td className="summary-cell">
                      <Link href={`/portal/calls/${c.id}`} style={{ color: "inherit" }}>
                        {c.summary || t("calls.detail.viewdetail")}
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <TestCallDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        defaultPhone={l.phone}
        defaultLeadName={l.name}
        onCallCreated={() => calls.reload()}
      />
    </>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="command-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function topOutcome(outcomes: string[]): string {
  const tally: Record<string, number> = {};
  for (const o of outcomes) tally[o] = (tally[o] || 0) + 1;
  let best = "";
  let bestCount = -1;
  for (const [k, v] of Object.entries(tally)) {
    if (v > bestCount) {
      best = k;
      bestCount = v;
    }
  }
  return best || "—";
}
