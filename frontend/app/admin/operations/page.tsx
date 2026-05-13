import { api } from "../../../lib/api";
import { StatCard } from "../../../components/stat-card";

export default async function OperationsPage() {
  const overview = await api.overview();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Operaciones</h1>
          <p className="subtle">Salud de telefonia, colas, IA y actividad global de la plataforma.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Ver logs</button>
          <button className="button">Abrir incidente</button>
        </div>
      </div>

      <div className="grid">
        <StatCard label="Llamadas" value={overview.callsToday} hint="Actividad demo actual" trend="ARI pendiente" />
        <StatCard label="En cola" value={overview.queuedCalls} hint="Pendientes de worker" trend="Redis listo" />
        <StatCard label="Campanas" value={overview.activeCampaigns} hint="Programadas" trend="Scheduler next" />
        <StatCard label="Callbacks" value={overview.callbacks} hint="Necesitan accion" trend="CRM next" />
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
            <div className="command-row">
              <span>Backend Go</span>
              <span className="status good">Activo local</span>
            </div>
            <div className="command-row">
              <span>Asterisk ARI</span>
              <span className="status warn">Sin worker</span>
            </div>
            <div className="command-row">
              <span>Voice agent</span>
              <span className="status warn">Pendiente</span>
            </div>
          </div>
        </section>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Latencia</p>
              <h2>Pipeline objetivo</h2>
            </div>
            <span className="chip">Realtime</span>
          </div>
          <div className="metric-list">
            <div>
              <div className="metric-item">
                <span>Audio in/out</span>
                <strong>250 ms</strong>
              </div>
              <div className="meter"><span style={{ width: "32%" }} /></div>
            </div>
            <div>
              <div className="metric-item">
                <span>Turn taking</span>
                <strong>450 ms</strong>
              </div>
              <div className="meter"><span style={{ width: "48%" }} /></div>
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
