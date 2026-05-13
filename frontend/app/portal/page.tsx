"use client";

import { useState } from "react";
import { ArrowDownRight, ArrowUpRight, Minus } from "lucide-react";
import { StatCard } from "../../components/stat-card";
import { TestCallDrawer } from "../../components/test-call-drawer";
import { DailyBars, HBars } from "../../components/charts";
import { api } from "../../lib/api";
import { useAuth, useTenantScope } from "../../lib/auth-context";
import { useResource } from "../../lib/use-resource";

export default function PortalDashboard() {
  const { user } = useAuth();
  const tenant = useTenantScope();
  const overview = useResource(() => api.overview(tenant), [tenant]);
  const analytics = useResource(() => api.analytics(tenant), [tenant]);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const trend = computeTrend(analytics.data?.totalsLast7 ?? 0, analytics.data?.totalsPrev7 ?? 0);

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Centro de llamadas IA</h1>
          <p className="subtle">
            Controla leads, bots, campañas y resultados desde una vista pensada para operar todos los días.
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

      <section className="hero-panel">
        <div className="command-panel">
          <p className="eyebrow">Estado de operación</p>
          <h2>Postgres conectado, ARI listo para originar</h2>
          <p className="subtle">
            Los datos vienen de Postgres y la auth aisla por tenant. Activa ARI en .env y configura el trunk SIP para
            lanzar llamadas reales.
          </p>
          <div className="filter-row">
            <span className="chip">Usuario: {user?.email}</span>
            <span className="chip">Rol: {user?.role}</span>
            <span className="chip">Tenant: {tenant || user?.tenantId || "—"}</span>
          </div>
        </div>
        <div className="panel">
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
        </div>
      </section>

      <div className="grid">
        <StatCard
          label="Llamadas hoy"
          value={overview.data?.callsToday ?? "—"}
          hint="Incluye completadas y en cola"
          trend={overview.loading ? "Cargando…" : "Vivo desde Postgres"}
        />
        <StatCard
          label="Leads calificados"
          value={overview.data?.qualifiedLeads ?? "—"}
          hint="Listos para seguimiento humano"
          trend="Alta intención"
        />
        <StatCard
          label="Callbacks"
          value={overview.data?.callbacks ?? "—"}
          hint="Pendientes de reagendar"
          trend="Acción requerida"
        />
        <StatCard
          label="Campañas activas"
          value={overview.data?.activeCampaigns ?? "—"}
          hint={`${overview.data?.queuedCalls ?? 0} llamadas en cola`}
          trend={overview.data?.queuedCalls ? "Cola con llamadas" : "Sin actividad"}
        />
      </div>

      {overview.error ? <div className="form-error" style={{ marginTop: 16 }}>Error: {overview.error}</div> : null}

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
