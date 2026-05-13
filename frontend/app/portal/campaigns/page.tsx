import { api, statusClass } from "../../../lib/api";

export default async function CampaignsPage() {
  const campaigns = await api.campaigns();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Campañas</h1>
          <p className="subtle">Programacion, cadencia y volumen de llamadas.</p>
        </div>
        <button className="button">Programar campaña</button>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Campaña</th>
              <th>Estado</th>
              <th>Horario</th>
              <th>Leads</th>
              <th>Intentos</th>
            </tr>
          </thead>
          <tbody>
            {campaigns.map((campaign) => (
              <tr key={campaign.id}>
                <td>{campaign.name}</td>
                <td><span className={statusClass(campaign.status)}>{campaign.status}</span></td>
                <td>{campaign.schedule}</td>
                <td>{campaign.leadCount}</td>
                <td>{campaign.maxAttempts}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

