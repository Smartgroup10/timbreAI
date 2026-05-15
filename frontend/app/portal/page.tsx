"use client";

import { useEffect, useState } from "react";
import { ArrowDownRight, ArrowUpRight, Minus, Phone, PhoneCall } from "lucide-react";
import { StatCard } from "../../components/stat-card";
import { TestCallDrawer } from "../../components/test-call-drawer";
import { DailyBars, HBars } from "../../components/charts";
import { api, Call, formatBytes, formatCostCents, statusClass } from "../../lib/api";
import { useTenantScope } from "../../lib/auth-context";
import { useResource } from "../../lib/use-resource";
import { useT, useStatusLabel } from "../../lib/i18n";

const LIVE_STATUSES = new Set(["queued", "dialing", "in_progress", "answered"]);

export default function PortalDashboard() {
  const tenant = useTenantScope();
  const t = useT();
  const statusLabel = useStatusLabel();
  const overview = useResource(() => api.overview(tenant), [tenant]);
  const analytics = useResource(() => api.analytics(tenant), [tenant]);
  const recordingUsage = useResource(() => api.recordingsUsage(tenant), [tenant]);
  const [drawerOpen, setDrawerOpen] = useState(false);

  // Llamadas en curso AHORA (queued/dialing/in_progress). Refrescamos cada 5s
  // — la única vista en la app que justifica polling rápido porque el operador
  // está mirando si su última campaña está marcando.
  const [liveCalls, setLiveCalls] = useState<Call[]>([]);
  const [recentCalls, setRecentCalls] = useState<Call[]>([]);
  useEffect(() => {
    let cancelled = false;
    async function refresh() {
      try {
        const all = await api.calls(tenant);
        if (cancelled) return;
        setLiveCalls(all.filter((c) => LIVE_STATUSES.has(c.status)).slice(0, 10));
        setRecentCalls(all.filter((c) => !LIVE_STATUSES.has(c.status)).slice(0, 6));
      } catch {
        /* ignore */
      }
    }
    refresh();
    const id = setInterval(refresh, 5_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [tenant]);

  const trend = computeTrend(analytics.data?.totalsLast7 ?? 0, analytics.data?.totalsPrev7 ?? 0, t);

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("portal.title")}</h1>
          <p className="subtle">{t("portal.subtitle")}</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setDrawerOpen(true)}>
            {t("portal.testcall")}
          </button>
          <a className="button" href="/portal/campaigns">
            {t("portal.newcampaign")}
          </a>
        </div>
      </div>

      <div className="grid">
        <StatCard
          label={t("portal.stat.live")}
          value={liveCalls.length}
          hint={liveCalls.length === 0 ? t("portal.stat.live.empty") : t("portal.stat.live.refresh")}
          trend={liveCalls.length > 0 ? t("portal.trend.live") : ""}
        />
        <StatCard
          label={t("portal.stat.today")}
          value={overview.data?.callsToday ?? "—"}
          hint={t("portal.stat.today.hint")}
          trend={overview.loading ? t("portal.trend.loading") : ""}
        />
        <StatCard
          label={t("portal.stat.qualified")}
          value={overview.data?.qualifiedLeads ?? "—"}
          hint={t("portal.stat.qualified.hint")}
          trend={t("portal.stat.qualified.trend")}
        />
        <StatCard
          label={t("portal.stat.activecampaigns")}
          value={overview.data?.activeCampaigns ?? "—"}
          hint={t("portal.stat.queued", { n: overview.data?.queuedCalls ?? 0 })}
          trend={overview.data?.queuedCalls ? t("portal.stat.queued.hasqueue") : ""}
        />
        <StatCard
          label={t("portal.stat.cost")}
          value={formatCostCents(analytics.data?.totalCostCents)}
          hint={t("portal.stat.cost.hint")}
          trend={
            analytics.data?.costByProvider && analytics.data.costByProvider.length > 0
              ? analytics.data.costByProvider
                  .map((c) => `${c.provider}: ${formatCostCents(c.costCents)}`)
                  .join(" · ")
              : ""
          }
        />
        <StatCard
          label={t("portal.stat.storage")}
          value={formatBytes(recordingUsage.data?.totalBytes)}
          hint={t("portal.stat.storage.hint", {
            size: formatBytes(recordingUsage.data?.totalBytes),
            n: recordingUsage.data?.count ?? 0,
          })}
          trend={""}
        />
      </div>

      {overview.error ? <div className="form-error" style={{ marginTop: 16 }}>Error: {overview.error}</div> : null}

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">
                <PhoneCall aria-hidden="true" style={{ verticalAlign: "middle", marginRight: 4 }} />
                {t("portal.live.eyebrow")}
              </p>
              <h2>{t("portal.live.title")}</h2>
            </div>
            {liveCalls.length > 0 ? (
              <span className="status good">{t("portal.live.count", { n: liveCalls.length })}</span>
            ) : null}
          </div>
          {liveCalls.length === 0 ? (
            <p className="subtle">{t("portal.live.empty")}</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>{t("col.lead")}</th>
                  <th>{t("col.phone")}</th>
                  <th>{t("col.status")}</th>
                </tr>
              </thead>
              <tbody>
                {liveCalls.map((c) => (
                  <tr key={c.id}>
                    <td>{c.leadName || "—"}</td>
                    <td>
                      <code className="mono">{c.phone}</code>
                    </td>
                    <td>
                      <span className={statusClass(c.status)}>{statusLabel(c.status)}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("portal.charts.7d")}</p>
              <h2>
                {t("portal.calls.unit", { n: analytics.data?.totalsLast7 ?? "—" })}{" "}
                <span className={`stat-trend trend-${trend.dir}`}>
                  {trend.icon} {trend.label}
                </span>
              </h2>
            </div>
            <span className="chip">{analytics.data?.timezone || "UTC"}</span>
          </div>
          <DailyBars data={analytics.data?.last7Days ?? []} />
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">
              <Phone aria-hidden="true" style={{ verticalAlign: "middle", marginRight: 4 }} />
              {t("portal.recent.eyebrow")}
            </p>
            <h2>{t("portal.recent.title")}</h2>
          </div>
          <a className="button ghost compact" href="/portal/calls">
            {t("portal.recent.viewall")}
          </a>
        </div>
        {recentCalls.length === 0 ? (
          <p className="subtle">{t("portal.recent.empty")}</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>{t("col.lead")}</th>
                <th>{t("col.phone")}</th>
                <th>{t("col.campaign")}</th>
                <th>{t("col.outcome")}</th>
                <th>{t("col.duration")}</th>
              </tr>
            </thead>
            <tbody>
              {recentCalls.map((c) => (
                <tr key={c.id}>
                  <td>{c.leadName || "—"}</td>
                  <td>
                    <code className="mono">{c.phone}</code>
                  </td>
                  <td>{c.campaign || "—"}</td>
                  <td>
                    <span className={statusClass(c.outcome)}>{statusLabel(c.outcome)}</span>
                  </td>
                  <td>{c.durationSec > 0 ? `${c.durationSec}s` : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("portal.charts.outcomes.eyebrow")}</p>
              <h2>{t("portal.charts.outcomes.title")}</h2>
            </div>
          </div>
          <HBars data={analytics.data?.outcomes ?? []} />
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("portal.charts.statuses.eyebrow")}</p>
              <h2>{t("portal.charts.statuses.title")}</h2>
            </div>
          </div>
          <HBars data={analytics.data?.statuses ?? []} accent="var(--accent)" />
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("portal.charts.bots.eyebrow")}</p>
              <h2>{t("portal.charts.bots.title")}</h2>
            </div>
          </div>
          <HBars data={analytics.data?.topBots ?? []} accent="var(--accent-strong)" />
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("portal.charts.campaigns.eyebrow")}</p>
              <h2>{t("portal.charts.campaigns.title")}</h2>
            </div>
          </div>
          <HBars data={analytics.data?.topCampaigns ?? []} accent="#6366f1" />
        </section>
      </div>

      <TestCallDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </>
  );
}

function computeTrend(now: number, prev: number, t: (k: string, vars?: Record<string, string | number>) => string) {
  if (prev === 0) {
    return {
      dir: "flat" as const,
      icon: <Minus aria-hidden="true" />,
      label: now > 0 ? t("portal.trend.nobase") : t("portal.trend.zero"),
    };
  }
  const pct = Math.round(((now - prev) / prev) * 100);
  if (pct > 0) return { dir: "up" as const, icon: <ArrowUpRight aria-hidden="true" />, label: t("portal.trend.up", { pct }) };
  if (pct < 0) return { dir: "down" as const, icon: <ArrowDownRight aria-hidden="true" />, label: t("portal.trend.down", { pct }) };
  return { dir: "flat" as const, icon: <Minus aria-hidden="true" />, label: t("portal.trend.flat") };
}
