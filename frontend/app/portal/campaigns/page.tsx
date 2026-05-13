import { api, statusClass } from "../../../lib/api";

export default async function CampaignsPage() {
  const campaigns = await api.campaigns();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Campanas</h1>
          <p className="subtle">Programacion, cadencia, volumen y control de llamadas por bot.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Plantillas</button>
          <button className="button">Programar campana</button>
        </div>
      </div>

      <div className="grid two" style={{ marginBottom: 16 }}>
        {campaigns.map((campaign) => (
          <section className="panel" key={campaign.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">Campana</p>
                <h2>{campaign.name}</h2>
              </div>
              <span className={statusClass(campaign.status)}>{campaign.status}</span>
            </div>
            <div className="command-strip">
              <div className="command-row">
                <span>Horario</span>
                <strong>{campaign.schedule}</strong>
              </div>
              <div className="command-row">
                <span>Leads</span>
                <strong>{campaign.leadCount}</strong>
              </div>
              <div className="command-row">
                <span>Intentos maximos</span>
                <strong>{campaign.maxAttempts}</strong>
              </div>
            </div>
          </section>
        ))}
      </div>

      <section className="panel">
        <div className="panel-header">
          <div>
            <p className="eyebrow">Control de lanzamiento</p>
            <h2>Reglas antes de llamar</h2>
          </div>
          <span className="status warn">Requiere validacion</span>
        </div>
        <div className="grid three">
          <div>
            <h3>Consentimiento</h3>
            <p className="subtle">Cruzar cada lead con fuente, opt-out y base de contacto.</p>
          </div>
          <div>
            <h3>Horario</h3>
            <p className="subtle">Respetar zona horaria del tenant y ventanas configuradas.</p>
          </div>
          <div>
            <h3>Volumen</h3>
            <p className="subtle">Limites diarios por cliente, campana y numero de salida.</p>
          </div>
        </div>
      </section>
    </>
  );
}
