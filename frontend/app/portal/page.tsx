"use client";

import { useEffect, useState } from "react";
import { ArrowDownRight, ArrowUpRight, Minus, Phone, PhoneCall } from "lucide-react";
import { StatCard } from "../../components/stat-card";
import { TestCallDrawer } from "../../components/test-call-drawer";
import { DailyBars, HBars } from "../../components/charts";
import { api, Call, statusClass } from "../../lib/api";
import { useTenantScope } from "../../lib/auth-context";
import { useResource } from "../../lib/use-resource";

const LIVE_STATUSES = new Set(["queued", "dialing", "in_progress", "answered"]);

export default function PortalDashboard() {
  const tenant = useTenantScope();
  const overview = useResource(() => api.overview(tenant), [tenant]);
  const analytics = useResource(() => api.analytics(tenant), [tenant]);
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

  const trend = computeTrend(analytics.data?.totalsLast7 ?? 0, analytics.data?.totalsPrev7 ?? 0);

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Centro de llamadas IA</h1>
          <p className="subtle">
            Vista en vivo de campañas, bots y resultados. Lo que esté pasando ahora aparece arriba.
          </p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setDrawerOpen(true)}>
            Llamada de prueba
          </button>
          <a className="button" href="/portal/campaigns">
            Nueva campaña
          </a>
        </div>
      </div>

      <div className="grid">
        <StatCard
          label="En curso ahora"
          value={liveCalls.length}
          hint={liveCalls.length === 0 ? "Sin llamadas activas" : "Refresca cada 5s"}
          trend={liveCalls.length > 0 ? "Live" : ""}
        />
        <StatCard
          label="Llamadas hoy"
          value={overview.data?.callsToday ?? "—"}
          hint="Total iniciadas en las últimas 24h"
          trend={overview.loading ? "Cargando…" : ""}
        />
        <StatCard
          label="Leads calificados"
          value={overview.data?.qualifiedLeads ?? "—"}
          hint="Outcome=qualified"
          trend="Listos para seguimiento humano"
        />
        <StatCard
          label="Campañas activas"
          value={overview.data?.activeCampaigns ?? "—"}
          hint={`${overview.data?.queuedCalls ?? 0} llamadas en cola`}
          trend={overview.data?.queuedCalls ? "Cola con llamadas" : ""}
        />
      </div>

      {overview.error ? <div className="form-error" style={{ marginTop: 16 }}>Error: {overview.error}</div> : null}

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">
                <PhoneCall aria-hidden="true" style={{ verticalAlign: "middle", marginRight: 4 }} />
                En vivo
              </p>
              <h2>Llamadas activas ahora</h2>
            </div>
            {liveCalls.length > 0 ? <span className="status good">{liveCalls.length} en curso</span> : null}
          </div>
          {liveCalls.length === 0 ? (
            <p className="subtle">
              Ninguna llamada en marcha. Lanza una campaña desde <a href="/portal/campaigns">Campañas</a> o usa{" "}
              el botón <strong>Llamada de prueba</strong> arriba.
            </p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Lead</th>
                  <th>Teléfono</th>
                  <th>Status</th>
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
                      <span className={statusClass(c.status)}>{c.status}</span>
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
              <p className="eyebrow">7 días</p>
              <h2>
                {analytics.data?.totalsLast7 ?? "—"} llamadas{" "}
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
              Recientes
            </p>
            <h2>Últimas llamadas finalizadas</h2>
          </div>
          <a className="button ghost compact" href="/portal/calls">
            Ver todas
          </a>
        </div>
        {recentCalls.length === 0 ? (
          <p className="subtle">Aún no hay llamadas finalizadas en este tenant.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Lead</th>
                <th>Teléfono</th>
                <th>Campaña</th>
                <th>Outcome</th>
                <th>Duración</th>
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
                    <span className={statusClass(c.outcome)}>{c.outcome || "—"}</span>
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
              <p className="eyebrow">Outcomes</p>
              <h2>Últimos 30 días</h2>
            </div>
          </div>
          <HBars data={analytics.data?.outcomes ?? []} />
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Status</p>
              <h2>Distribución</h2>
            </div>
          </div>
          <HBars data={analytics.data?.statuses ?? []} accent="var(--accent)" />
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Top bots</p>
              <h2>Volumen 30 días</h2>
            </div>
          </div>
          <HBars data={analytics.data?.topBots ?? []} accent="var(--accent-strong)" />
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Top campañas</p>
              <h2>Volumen 30 días</h2>
            </div>
          </div>
          <HBars data={analytics.data?.topCampaigns ?? []} accent="#6366f1" />
        </section>
      </div>

      <TestCallDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </>
  );
}

function computeTrend(now: number, prev: number) {
  if (prev === 0) {
    return { dir: "flat" as const, icon: <Minus aria-hidden="true" />, label: now > 0 ? "Sin base comparable" : "0%" };
  }
  const pct = Math.round(((now - prev) / prev) * 100);
  if (pct > 0) return { dir: "up" as const, icon: <ArrowUpRight aria-hidden="true" />, label: `+${pct}% vs 7d ant.` };
  if (pct < 0) return { dir: "down" as const, icon: <ArrowDownRight aria-hidden="true" />, label: `${pct}% vs 7d ant.` };
  return { dir: "flat" as const, icon: <Minus aria-hidden="true" />, label: "Sin cambios" };
}
