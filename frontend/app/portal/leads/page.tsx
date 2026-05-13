import { api, statusClass } from "../../../lib/api";

export default async function LeadsPage() {
  const leads = await api.leads();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Leads</h1>
          <p className="subtle">Contactos disponibles para campañas y llamadas de seguimiento.</p>
        </div>
        <button className="button">Importar CSV</button>
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
              <th>Consentimiento</th>
            </tr>
          </thead>
          <tbody>
            {leads.map((lead) => (
              <tr key={lead.id}>
                <td>{lead.name}</td>
                <td>{lead.phone}</td>
                <td>{lead.email}</td>
                <td>{lead.type}</td>
                <td><span className={statusClass(lead.status)}>{lead.status}</span></td>
                <td>{lead.consent}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

