import { api } from "../../lib/api";
import { StatCard } from "../../components/stat-card";

export default async function PortalDashboard() {
  const overview = await api.overview();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Dashboard</h1>
          <p className="subtle">Estado operativo de llamadas, campañas y oportunidades.</p>
        </div>
        <button className="button">Nueva campaña</button>
      </div>
      <div className="grid">
        <StatCard label="Llamadas hoy" value={overview.callsToday} hint="Incluye completadas y en cola" />
        <StatCard label="Leads calificados" value={overview.qualifiedLeads} hint="Listos para seguimiento humano" />
        <StatCard label="Callbacks" value={overview.callbacks} hint="Pendientes de reagendar" />
        <StatCard label="Campañas activas" value={overview.activeCampaigns} hint={`${overview.queuedCalls} llamadas en cola`} />
      </div>
      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <h2>Prioridades</h2>
          <p className="subtle">Completar datos de propiedades, revisar consentimientos y activar una llamada de prueba antes de usar un trunk real.</p>
        </section>
        <section className="panel">
          <h2>Compliance</h2>
          <p className="subtle">El bot debe identificarse como asistente automatizado y respetar opt-out desde el primer turno.</p>
        </section>
      </div>
    </>
  );
}

