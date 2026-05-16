"use client";

import { use, useState } from "react";
import Link from "next/link";
import { ArrowLeft, PhoneCall, PhoneOff, User } from "lucide-react";
import { useConfirm } from "../../../../components/confirm";
import { TestCallDrawer } from "../../../../components/test-call-drawer";
import { useToast } from "../../../../components/toast";
import { api, ApiError, formatCostCents, statusClass } from "../../../../lib/api";
import { useTenantScope } from "../../../../lib/auth-context";
import { useResource } from "../../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../../lib/i18n";

export default function CallDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const tenant = useTenantScope();
  const t = useT();
  const statusLabel = useStatusLabel();
  const toast = useToast();
  const confirm = useConfirm();
  const call = useResource(() => api.getCall(id, tenant), [id, tenant]);
  const transcripts = useResource(() => api.callTranscripts(id, tenant), [id, tenant]);
  const [drawerOpen, setDrawerOpen] = useState(false);

  async function handleAddToDNC(phone: string) {
    const ok = await confirm({
      title: t("calls.detail.dnc.confirm.title"),
      description: t("calls.detail.dnc.confirm.desc", { phone }),
      variant: "danger",
      confirmLabel: t("calls.detail.dnc.confirm.cta"),
    });
    if (!ok) return;
    try {
      await api.addDNC({ phone, reason: t("calls.detail.dnc.reason") }, tenant);
      toast.push(t("calls.detail.dnc.toast.added"), "success");
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("calls.detail.dnc.toast.failed", { err: code }), "danger");
    }
  }

  if (call.loading) {
    return <div className="empty-state">{t("calls.detail.loading")}</div>;
  }
  if (call.error) {
    return <div className="empty-state danger">{t("g.error")}: {call.error}</div>;
  }
  if (!call.data) {
    return <div className="empty-state">{t("calls.detail.notfound")}</div>;
  }

  const c = call.data;
  const lines = transcripts.data ?? [];

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
          <Link href="/portal/calls" className="button ghost compact" style={{ marginBottom: 8 }}>
            <ArrowLeft aria-hidden="true" />
            <span>{t("calls.detail.back")}</span>
          </Link>
          <p className="eyebrow">{t("calls.detail.eyebrow")}</p>
          <h1>{c.leadName || c.phone}</h1>
          <p className="subtle">
            <code className="mono">{c.id}</code>
          </p>
        </div>
        <div className="actions" style={{ gap: 8, flexWrap: "wrap" }}>
          <span className={statusClass(c.status)}>{statusLabel(c.status)}</span>
          <span className="chip">{statusLabel(c.outcome)}</span>
          {c.leadId ? (
            <Link href={`/portal/leads/${c.leadId}`} className="button ghost compact">
              <User aria-hidden="true" />
              <span>{t("calls.detail.action.viewlead")}</span>
            </Link>
          ) : null}
          <button className="button compact" onClick={() => setDrawerOpen(true)}>
            <PhoneCall aria-hidden="true" />
            <span>{t("calls.detail.action.recall")}</span>
          </button>
          <button className="button ghost compact" onClick={() => handleAddToDNC(c.phone)}>
            <PhoneOff aria-hidden="true" />
            <span>{t("calls.detail.action.dnc")}</span>
          </button>
        </div>
      </div>

      <div className="grid two">
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("calls.detail.details.eyebrow")}</p>
              <h2>{t("calls.detail.details.title")}</h2>
            </div>
          </div>
          <div className="command-strip">
            <Row label={t("col.phone")} value={<code className="mono">{c.phone}</code>} />
            <Row label={t("calls.detail.lead")} value={c.leadName || "—"} />
            <Row label={t("calls.detail.campaign")} value={c.campaign || "—"} />
            <Row label={t("calls.detail.duration")} value={formatDuration(c.durationSec)} />
            <Row
              label={t("col.cost")}
              value={
                <span title={t("cost.hint")}>
                  {formatCostCents(c.costCents)}
                  {c.provider ? <span className="subtle"> · {c.provider}</span> : null}
                </span>
              }
            />
            <Row label={t("calls.detail.start")} value={c.startedAt ? new Date(c.startedAt).toLocaleString() : "—"} />
            <Row label={t("calls.detail.end")} value={c.endedAt ? new Date(c.endedAt).toLocaleString() : "—"} />
            <Row label={t("calls.detail.channel")} value={<code className="mono">{c.channelId || "—"}</code>} />
            <Row
              label={t("calls.detail.voicesession")}
              value={c.voiceSessionId ? <code className="mono">{c.voiceSessionId}</code> : <span className="subtle">{t("calls.detail.voicesession.empty")}</span>}
            />
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("calls.detail.summary.eyebrow")}</p>
              <h2>{t("calls.detail.summary.title")}</h2>
            </div>
          </div>
          <p className="subtle">{c.summary || t("calls.detail.summary.empty")}</p>

          {c.recordingUrl ? (
            <div style={{ marginTop: 14 }}>
              <p className="eyebrow">{t("calls.detail.recording")}</p>
              <audio controls src={c.recordingUrl} style={{ width: "100%" }} />
            </div>
          ) : null}
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("calls.detail.conversation.eyebrow")}</p>
            <h2>{t("calls.detail.conversation.title", { n: lines.length })}</h2>
          </div>
          <button className="button secondary compact" onClick={() => transcripts.reload()}>
            {t("calls.detail.refresh")}
          </button>
        </div>
        {transcripts.loading ? (
          <div className="empty-state">{t("g.loading")}</div>
        ) : lines.length === 0 ? (
          <div className="empty-state">{t("calls.detail.conversation.empty")}</div>
        ) : (
          <div className="transcript">
            {lines.map((line) => (
              <div key={line.id} className={`transcript-line transcript-${line.role}`}>
                <span className="transcript-role">{line.role}</span>
                <span className="transcript-text">{line.text}</span>
                <time className="transcript-time">{new Date(line.occurredAt).toLocaleTimeString()}</time>
              </div>
            ))}
          </div>
        )}
      </section>

      <TestCallDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        defaultPhone={c.phone}
        defaultLeadName={c.leadName || ""}
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
