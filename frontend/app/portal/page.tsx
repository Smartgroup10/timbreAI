"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowDownRight, ArrowUpRight, ChevronDown, ChevronRight, Minus, Phone, PhoneCall, Radio } from "lucide-react";
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
  const [trendsOpen, setTrendsOpen] = useState(false);
  const [distrOpen, setDistrOpen] = useState(false);

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

      {/* HERO — lo que pasa ahora mismo. Si hay llamadas activas, cards
       *   grandes con cada una; si no, mensaje + acción CTA. */}
      <section className="panel hero-live" style={{ marginBottom: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">
              <Radio aria-hidden="true" style={{ verticalAlign: "middle", marginRight: 4 }} />
              {t("portal.hero.eyebrow")}
            </p>
            <h2>
              {liveCalls.length === 0
                ? t("portal.hero.empty.title")
                : t("portal.hero.count", { n: liveCalls.length })}
            </h2>
          </div>
          {liveCalls.length > 0 ? (
            <a className="button ghost compact" href="/portal/calls">
              {t("portal.hero.viewall")}
            </a>
          ) : (
            <button className="button compact" onClick={() => setDrawerOpen(true)}>
              {t("portal.testcall")}
            </button>
          )}
        </div>
        {liveCalls.length === 0 ? (
          <p className="subtle" style={{ margin: 0 }}>{t("portal.hero.empty.desc")}</p>
        ) : (
          <div className="live-cards">
            {liveCalls.map((c) => (
              <Link key={c.id} href={`/portal/calls/${c.id}`} className="live-card">
                <div className="live-card-head">
                  <strong>{c.leadName || c.phone}</strong>
                  <span className={statusClass(c.status)}>{statusLabel(c.status)}</span>
                </div>
                <code className="mono live-card-phone">{c.phone}</code>
                <div className="live-card-meta">
                  <span>{c.campaign || t("portal.hero.nocampaign")}</span>
                  <span>{c.durationSec > 0 ? `${c.durationSec}s` : "—"}</span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </section>

      {/* KPIs del día — fila compacta de 4 cards. */}
      <div className="grid">
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
      </div>

      {overview.error ? <div className="form-error" style={{ marginTop: 16 }}>Error: {overview.error}</div> : null}

      {/* Últimas llamadas — siempre visible: el operador típicamente
       *   quiere ver "qué pasó en las últimas N llamadas". */}
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
                  <td>
                    <Link href={`/portal/calls/${c.id}`} style={{ color: "inherit" }}>
                      {c.leadName || "—"}
                    </Link>
                  </td>
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

      {/* Tendencias 7d — colapsado por defecto. Para los que quieran
       *   ver evolución; el operador típico no lo abre cada día. */}
      <section className="panel collapsible" style={{ marginTop: 16 }}>
        <button
          type="button"
          className="panel-header collapsible-trigger"
          onClick={() => setTrendsOpen((v) => !v)}
          aria-expanded={trendsOpen}
        >
          <div>
            <p className="eyebrow">{t("portal.charts.7d")}</p>
            <h2>
              {t("portal.calls.unit", { n: analytics.data?.totalsLast7 ?? "—" })}{" "}
              <span className={`stat-trend trend-${trend.dir}`}>
                {trend.icon} {trend.label}
              </span>
            </h2>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span className="chip">{analytics.data?.timezone || "UTC"}</span>
            <span className="collapsible-caret">
              {trendsOpen ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
            </span>
          </div>
        </button>
        {trendsOpen ? <DailyBars data={analytics.data?.last7Days ?? []} /> : null}
      </section>

      {/* Distribuciones — outcomes/statuses/bots/campaigns. Colapsadas. */}
      <section className="panel collapsible" style={{ marginTop: 16 }}>
        <button
          type="button"
          className="panel-header collapsible-trigger"
          onClick={() => setDistrOpen((v) => !v)}
          aria-expanded={distrOpen}
        >
          <div>
            <p className="eyebrow">{t("portal.distr.eyebrow")}</p>
            <h2>{t("portal.distr.title")}</h2>
          </div>
          <span className="collapsible-caret">
            {distrOpen ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
          </span>
        </button>
        {distrOpen ? (
          <div className="grid two">
            <div>
              <p className="eyebrow">{t("portal.charts.outcomes.title")}</p>
              <HBars data={analytics.data?.outcomes ?? []} />
            </div>
            <div>
              <p className="eyebrow">{t("portal.charts.statuses.title")}</p>
              <HBars data={analytics.data?.statuses ?? []} accent="var(--accent)" />
            </div>
            <div>
              <p className="eyebrow">{t("portal.charts.bots.title")}</p>
              <HBars data={analytics.data?.topBots ?? []} accent="var(--accent-strong)" />
            </div>
            <div>
              <p className="eyebrow">{t("portal.charts.campaigns.title")}</p>
              <HBars data={analytics.data?.topCampaigns ?? []} accent="#6366f1" />
            </div>
          </div>
        ) : null}
      </section>

      {/* Storage card al final — info más operativa que estratégica. */}
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("portal.stat.storage")}</p>
            <h2>{formatBytes(recordingUsage.data?.totalBytes)}</h2>
          </div>
          <span className="subtle">
            {t("portal.stat.storage.hint", {
              size: formatBytes(recordingUsage.data?.totalBytes),
              n: recordingUsage.data?.count ?? 0,
            })}
          </span>
        </div>
      </section>

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
