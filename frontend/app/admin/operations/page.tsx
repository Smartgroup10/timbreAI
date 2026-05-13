import { api } from "../../../lib/api";
import { StatCard } from "../../../components/stat-card";

export default async function OperationsPage() {
  const overview = await api.overview();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Admin interno</p>
          <h1>Operaciones</h1>
          <p className="subtle">Vista de salud para telefonia, colas y actividad global.</p>
        </div>
        <button className="button secondary">Ver logs</button>
      </div>
      <div className="grid">
        <StatCard label="Llamadas" value={overview.callsToday} hint="Actividad demo actual" />
        <StatCard label="En cola" value={overview.queuedCalls} hint="Pendientes de worker" />
        <StatCard label="Campañas" value={overview.activeCampaigns} hint="Programadas" />
        <StatCard label="Callbacks" value={overview.callbacks} hint="Necesitan accion" />
      </div>
      <section className="panel" style={{ marginTop: 16 }}>
        <h2>Estado de integraciones</h2>
        <p className="subtle">Asterisk ARI, Postgres y Redis estan definidos en Docker Compose. La conexion real de originate queda para la siguiente iteracion.</p>
      </section>
    </>
  );
}

