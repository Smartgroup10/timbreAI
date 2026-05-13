"use client";

import { StatCard } from "../../../components/stat-card";
import { api } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

export default function OperationsPage() {
  const { user } = useAuth();
  const overview = useResource(() => api.overview(), []);
  const ops = useResource(() => api.operations(), []);

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">Acceso restringido al rol platform_admin.</div>;
  }

  const ariEnabled = Boolean(ops.data?.ariEnabled);
  const trunkCount = Number(ops.data?.trunkCount ?? 0);
  const activeTrunks = Number(ops.data?.activeTrunks ?? 0);
  const voiceAgentReachable = Boolean(ops.data?.voiceAgentReachable);
  const voiceProviders = (ops.data?.voiceProviders as string[] | undefined) ?? [];
  const realProviders = voiceProviders.filter((p) => p !== "echo");

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Operaciones</h1>
          <p className="subtle">Salud de telefonia, colas, IA y actividad global de la plataforma.</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => overview.reload()}>
            Refrescar
          </button>
        </div>
      </div>

      <div className="grid">
        <StatCard
          label="Llamadas"
          value={overview.data?.callsToday ?? "—"}
          hint="Vivas en Postgres"
          trend={ariEnabled ? "ARI activo" : "ARI deshabilitado"}
        />
        <StatCard
          label="En cola"
          value={overview.data?.queuedCalls ?? "—"}
          hint="Pendientes de originar"
          trend={activeTrunks > 0 ? `${activeTrunks} trunk activos` : "Sin trunks"}
        />
        <StatCard
          label="Campanas"
          value={overview.data?.activeCampaigns ?? "—"}
          hint="Status scheduled"
          trend="Scheduler pendiente"
        />
        <StatCard
          label="Callbacks"
          value={overview.data?.callbacks ?? "—"}
          hint="Necesitan accion"
          trend="Manual por ahora"
        />
      </div>

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Infraestructura</p>
              <h2>Servicios definidos</h2>
            </div>
            <span className="status good">Compose OK</span>
          </div>
          <div className="command-strip">
            <OpRow label="Backend Go (Postgres)" ok positive="Conectado" />
            <OpRow label="Asterisk ARI" ok={ariEnabled} positive="Conectado" negative="Deshabilitado" />
            <OpRow
              label="Trunks SIP"
              ok={activeTrunks > 0}
              positive={`${activeTrunks}/${trunkCount} activos`}
              negative={trunkCount > 0 ? "Todos inactivos" : "Sin trunks (configura en /admin/trunks)"}
            />
            <OpRow
              label="Voice agent"
              ok={voiceAgentReachable}
              positive={
                realProviders.length > 0
                  ? `${realProviders.length} providers reales (${realProviders.join(", ")}) + echo`
                  : "Echo provider (sin API keys reales)"
              }
              negative="No alcanzable"
            />
          </div>
        </section>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Pipeline objetivo</p>
              <h2>Latencia voz</h2>
            </div>
            <span className="chip">Realtime</span>
          </div>
          <div className="metric-list">
            <Meter label="Audio in/out" value="250 ms" pct={32} />
            <Meter label="Turn taking" value="450 ms" pct={48} />
            <Meter label="ASR + LLM + TTS" value="600 ms" pct={64} />
          </div>
          <p className="subtle" style={{ marginTop: 12 }}>
            Estos targets se miden cuando el voice agent este conectado al canal ARI via External Media.
          </p>
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Configuracion en vivo</p>
            <h2>Variables del backend</h2>
          </div>
          <span className="chip">Solo lectura</span>
        </div>
        <pre className="code-block">
{JSON.stringify(ops.data ?? {}, null, 2)}
        </pre>
      </section>
    </>
  );
}

function OpRow({ label, ok, positive, negative }: { label: string; ok: boolean; positive?: string; negative?: string }) {
  return (
    <div className="command-row">
      <span>{label}</span>
      <span className={ok ? "status good" : "status warn"}>{ok ? positive || "OK" : negative || "Pendiente"}</span>
    </div>
  );
}

function Meter({ label, value, pct }: { label: string; value: string; pct: number }) {
  return (
    <div>
      <div className="metric-item">
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
      <div className="meter">
        <span style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
