import { api } from "../../lib/api";
import { StatCard } from "../../components/stat-card";

export default async function PortalDashboard() {
  const overview = await api.overview();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Centro de llamadas IA</h1>
          <p className="subtle">
            Controla leads, bots, campanas y resultados desde una vista pensada para operar todos los dias.
          </p>
        </div>
        <div className="actions">
          <button className="button secondary">Llamada de prueba</button>
          <button className="button">Nueva campana</button>
        </div>
      </div>

      <section className="hero-panel">
        <div className="command-panel">
          <p className="eyebrow">Estado de operacion</p>
          <h2>Renter follow-up listo para pruebas controladas</h2>
          <p className="subtle">
            El bot tiene datos demo de propiedades, reglas basicas de compliance y cola simulada. El siguiente salto es conectar Postgres y ARI real.
          </p>
          <div className="filter-row">
            <span className="chip">Tenant: Atrium</span>
            <span className="chip">Canal: Voz</span>
            <span className="chip">Modo: Sandbox</span>
          </div>
        </div>
        <div className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Checklist</p>
              <h2>Antes del trunk</h2>
            </div>
            <span className="status warn">3 pendientes</span>
          </div>
          <div className="timeline">
            <div className="timeline-item">
              <div>
                <h3>Validar consentimiento</h3>
                <p className="subtle">Todo lead debe tener fuente y base de llamada.</p>
              </div>
            </div>
            <div className="timeline-item">
              <div>
                <h3>Activar worker ARI</h3>
                <p className="subtle">Origen de llamadas y eventos de canal.</p>
              </div>
            </div>
            <div className="timeline-item">
              <div>
                <h3>Prueba con extension interna</h3>
                <p className="subtle">Sin exponer SIP publico todavia.</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <div className="grid">
        <StatCard label="Llamadas hoy" value={overview.callsToday} hint="Incluye completadas y en cola" trend="+12% vs demo" />
        <StatCard label="Leads calificados" value={overview.qualifiedLeads} hint="Listos para seguimiento humano" trend="Alta intencion" />
        <StatCard label="Callbacks" value={overview.callbacks} hint="Pendientes de reagendar" trend="Accion requerida" />
        <StatCard label="Campanas activas" value={overview.activeCampaigns} hint={`${overview.queuedCalls} llamadas en cola`} trend="Sandbox" />
      </div>

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Conversion</p>
              <h2>Embudo operativo</h2>
            </div>
            <span className="status good">Saludable</span>
          </div>
          <div className="metric-list">
            <div>
              <div className="metric-item">
                <span>Contactados</span>
                <strong>68%</strong>
              </div>
              <div className="meter"><span style={{ width: "68%" }} /></div>
            </div>
            <div>
              <div className="metric-item">
                <span>Calificados</span>
                <strong>41%</strong>
              </div>
              <div className="meter"><span style={{ width: "41%" }} /></div>
            </div>
            <div>
              <div className="metric-item">
                <span>Transferidos</span>
                <strong>19%</strong>
              </div>
              <div className="meter"><span style={{ width: "19%" }} /></div>
            </div>
          </div>
        </section>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Compliance</p>
              <h2>Reglas del bot</h2>
            </div>
            <span className="status warn">Sandbox</span>
          </div>
          <div className="command-strip">
            <div className="command-row">
              <span>Identificacion como asistente IA</span>
              <span className="status good">Activo</span>
            </div>
            <div className="command-row">
              <span>Bloqueo de numeros opt-out</span>
              <span className="status warn">Pendiente DB</span>
            </div>
            <div className="command-row">
              <span>Transferencia por preguntas sensibles</span>
              <span className="status good">Definido</span>
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
