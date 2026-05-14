"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { PhoneCall } from "lucide-react";
import { EmptyState } from "../../../components/empty";
import { TableSkeleton } from "../../../components/skeleton";
import { TestCallDrawer } from "../../../components/test-call-drawer";
import { api, formatCostCents, statusClass } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useRealtime } from "../../../lib/use-realtime";
import { useResource } from "../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../lib/i18n";

const STATUS_FILTERS = ["all", "completed", "queued", "dialing", "failed", "skipped"] as const;
const POLL_MS = 10_000;

export default function CallsPage() {
  const tenant = useTenantScope();
  const t = useT();
  const statusLabel = useStatusLabel();
  const calls = useResource(() => api.calls(tenant), [tenant], { pollMs: POLL_MS });
  const [filter, setFilter] = useState<(typeof STATUS_FILTERS)[number]>("all");
  const [drawerOpen, setDrawerOpen] = useState(false);

  // Realtime push: cuando el backend confirma una llamada terminada,
  // refrescamos al instante en vez de esperar al próximo tick de polling.
  useRealtime((ev) => {
    if (ev.type === "call.finished" || ev.type === "call.updated" || ev.type === "call.created") {
      calls.reload();
    }
  });

  const filtered = useMemo(() => {
    const data = calls.data ?? [];
    if (filter === "all") return data;
    return data.filter((c) => c.status === filter);
  }, [calls.data, filter]);

  function formatDuration(sec: number) {
    if (!sec) return "—";
    if (sec < 60) return `${sec}s`;
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return s === 0 ? `${m}m` : `${m}m ${s}s`;
  }

  const hasAny = (calls.data?.length ?? 0) > 0;

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("calls.title")}</h1>
          <p className="subtle">{t("calls.subtitle")}</p>
        </div>
        <div className="actions">
          <span className="refresh-dot" aria-live="polite">
            {t("empty.live", { n: POLL_MS / 1000 })}
          </span>
          <button className="button secondary" disabled>
            {t("btn.export")}
          </button>
          <button className="button" onClick={() => setDrawerOpen(true)}>
            {t("portal.testcall")}
          </button>
        </div>
      </div>

      <div className="filter-row">
        {STATUS_FILTERS.map((opt) => (
          <button
            key={opt}
            className={`chip-button${filter === opt ? " active" : ""}`}
            onClick={() => setFilter(opt)}
          >
            {opt === "all" ? t("calls.filter.all") : statusLabel(opt)}
          </button>
        ))}
        <button className="chip-button" onClick={() => calls.reload()}>
          {t("btn.refresh")}
        </button>
      </div>

      {calls.loading ? (
        <TableSkeleton cols={8} rows={6} />
      ) : calls.error ? (
        <div className="empty-state danger">{t("g.error")}: {calls.error}</div>
      ) : !hasAny ? (
        <EmptyState
          icon={PhoneCall}
          title={t("calls.empty.full")}
          description={t("calls.empty.desc")}
          action={{ label: t("calls.btn.testcall"), onClick: () => setDrawerOpen(true) }}
          secondary={{ label: t("calls.btn.gocampaigns"), href: "/portal/campaigns" }}
        />
      ) : filtered.length === 0 ? (
        <EmptyState title={t("calls.empty.nomatch")} />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("col.lead")}</th>
                <th>{t("col.phone")}</th>
                <th>{t("col.campaign")}</th>
                <th>{t("col.status")}</th>
                <th>{t("col.outcome")}</th>
                <th>{t("col.duration")}</th>
                <th>{t("col.cost")}</th>
                <th>{t("col.channel")}</th>
                <th>{t("col.summary")}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((call) => (
                <tr key={call.id} style={{ cursor: "pointer" }}>
                  <td className="primary-cell">
                    <Link href={`/portal/calls/${call.id}`} style={{ color: "inherit" }}>
                      {call.leadName || "—"}
                    </Link>
                  </td>
                  <td>{call.phone}</td>
                  <td>{call.campaign || "—"}</td>
                  <td>
                    <span className={statusClass(call.status)}>{statusLabel(call.status)}</span>
                  </td>
                  <td>
                    <span className="chip">{statusLabel(call.outcome)}</span>
                  </td>
                  <td>{formatDuration(call.durationSec)}</td>
                  <td title={call.provider ? `${call.provider}` : undefined}>
                    {formatCostCents(call.costCents)}
                  </td>
                  <td>
                    <code className="mono">{call.channelId || "—"}</code>
                  </td>
                  <td className="summary-cell">
                    <Link href={`/portal/calls/${call.id}`} style={{ color: "inherit" }}>
                      {call.summary || t("calls.detail.viewdetail")}
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <TestCallDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onCallCreated={() => calls.reload()}
      />
    </>
  );
}
