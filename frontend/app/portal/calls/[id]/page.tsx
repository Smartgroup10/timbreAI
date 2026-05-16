"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, PhoneCall, PhoneOff, User } from "lucide-react";
import { useConfirm } from "../../../../components/confirm";
import { TestCallDrawer } from "../../../../components/test-call-drawer";
import { useToast } from "../../../../components/toast";
import { api, ApiError, CallUsage, formatCostCents, formatMicroCents, statusClass } from "../../../../lib/api";
import { useTenantScope } from "../../../../lib/auth-context";
import { useResource } from "../../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../../lib/i18n";

type DetailTab = "conversation" | "details" | "cost" | "recording";

const DETAIL_TABS: { id: DetailTab; labelKey: string }[] = [
  { id: "conversation", labelKey: "calls.detail.tab.conversation" },
  { id: "details", labelKey: "calls.detail.tab.details" },
  { id: "cost", labelKey: "calls.detail.tab.cost" },
  { id: "recording", labelKey: "calls.detail.tab.recording" },
];

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
  const [tab, setTab] = useState<DetailTab>("conversation");
  // Usage solo se fetcha cuando entras al tab de coste — ahorra una llamada
  // para la mayoría que solo quiere ver la transcripción.
  const [usage, setUsage] = useState<CallUsage | null>(null);
  const [usageLoading, setUsageLoading] = useState(false);

  useEffect(() => {
    if (tab !== "cost" || usage) return;
    setUsageLoading(true);
    api
      .billingCall(id, tenant)
      .then(setUsage)
      .catch(() => setUsage(null))
      .finally(() => setUsageLoading(false));
  }, [tab, id, tenant, usage]);

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

      {/* Resumen siempre visible — el operador necesita la info clave
       *   sin tener que cambiar de tab. */}
      {c.summary ? (
        <section className="panel" style={{ marginBottom: 16 }}>
          <p className="eyebrow">{t("calls.detail.summary.eyebrow")}</p>
          <p style={{ margin: "4px 0 0" }}>{c.summary}</p>
        </section>
      ) : null}

      <div className="filter-row" style={{ marginBottom: 16 }}>
        {DETAIL_TABS.map((tt) => (
          <button
            key={tt.id}
            type="button"
            className={`chip-button${tab === tt.id ? " active" : ""}`}
            onClick={() => setTab(tt.id)}
          >
            {t(tt.labelKey)}
          </button>
        ))}
      </div>

      {tab === "conversation" ? (
        <section className="panel">
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
      ) : null}

      {tab === "details" ? (
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
            <Row label={t("calls.detail.start")} value={c.startedAt ? new Date(c.startedAt).toLocaleString() : "—"} />
            <Row label={t("calls.detail.end")} value={c.endedAt ? new Date(c.endedAt).toLocaleString() : "—"} />
            <Row label={t("calls.detail.channel")} value={<code className="mono">{c.channelId || "—"}</code>} />
            <Row
              label={t("calls.detail.voicesession")}
              value={
                c.voiceSessionId ? (
                  <code className="mono">{c.voiceSessionId}</code>
                ) : (
                  <span className="subtle">{t("calls.detail.voicesession.empty")}</span>
                )
              }
            />
          </div>
        </section>
      ) : null}

      {tab === "cost" ? (
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("calls.detail.cost.eyebrow")}</p>
              <h2>
                {usage ? formatMicroCents(usage.totalMicroCents) : formatCostCents(c.costCents)}
                {c.provider ? <span className="subtle"> · {c.provider}</span> : null}
              </h2>
            </div>
          </div>
          {usageLoading ? (
            <div className="empty-state">{t("g.loading")}</div>
          ) : !usage ? (
            <p className="subtle">{t("calls.detail.cost.estimateonly")}</p>
          ) : (
            <div className="command-strip">
              <Row label={t("calls.detail.cost.stt")} value={`${usage.sttSeconds}s · ${formatMicroCents(usage.sttMicroCents)}`} />
              <Row
                label={t("calls.detail.cost.llm")}
                value={`${usage.llmInputTokens} in / ${usage.llmOutputTokens} out · ${formatMicroCents(usage.llmMicroCents)}`}
              />
              <Row label={t("calls.detail.cost.tts")} value={`${usage.ttsChars} chars · ${formatMicroCents(usage.ttsMicroCents)}`} />
              <Row label={t("calls.detail.cost.trunk")} value={formatMicroCents(usage.trunkMicroCents)} />
              <Row label={t("calls.detail.cost.other")} value={formatMicroCents(usage.otherMicroCents)} />
              <Row
                label={t("calls.detail.cost.total")}
                value={<strong>{formatMicroCents(usage.totalMicroCents)}</strong>}
              />
            </div>
          )}
        </section>
      ) : null}

      {tab === "recording" ? (
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("calls.detail.recording.eyebrow")}</p>
              <h2>{t("calls.detail.recording")}</h2>
            </div>
          </div>
          {c.recordingUrl ? (
            <audio controls src={c.recordingUrl} style={{ width: "100%" }} />
          ) : (
            <p className="subtle">{t("calls.detail.recording.empty")}</p>
          )}
        </section>
      ) : null}

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
