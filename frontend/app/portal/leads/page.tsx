import { api, statusClass } from "../../../lib/api";

export default async function LeadsPage() {
  const leads = await api.leads();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Leads</h1>
          <p className="subtle">Contactos disponibles para campanas, llamadas de seguimiento y handoff comercial.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Nuevo lead</button>
          <button className="button">Importar CSV</button>
        </div>
      </div>

      <div className="filter-row">
        <span className="chip">Renter</span>
        <span className="chip">Owner</span>
        <span className="chip">Con consentimiento</span>
        <span className="chip">Callback</span>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Nombre</th>
              <th>Telefono</th>
              <th>Email</th>
              <th>Tipo</th>
              <th>Estado</th>
              <th>Fuente</th>
              <th>Consentimiento</th>
            </tr>
          </thead>
          <tbody>
            {leads.map((lead) => (
              <tr key={lead.id}>
                <td className="primary-cell">{lead.name}</td>
                <td>{lead.phone}</td>
                <td>{lead.email}</td>
                <td><span className="chip">{lead.type}</span></td>
                <td><span className={statusClass(lead.status)}>{lead.status}</span></td>
                <td>{lead.source}</td>
                <td>{lead.consent}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
