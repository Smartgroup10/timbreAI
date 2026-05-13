import { api, statusClass } from "../../../lib/api";

export default async function CallsPage() {
  const calls = await api.calls();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Llamadas</h1>
          <p className="subtle">Historial operativo con resultado, duracion y resumen accionable del bot.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Exportar</button>
          <button className="button">Llamada de prueba</button>
        </div>
      </div>

      <div className="filter-row">
        <span className="chip">Todas</span>
        <span className="chip">Completadas</span>
        <span className="chip">Callbacks</span>
        <span className="chip">En cola</span>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Lead</th>
              <th>Telefono</th>
              <th>Campana</th>
              <th>Estado</th>
              <th>Resultado</th>
              <th>Duracion</th>
              <th>Resumen</th>
            </tr>
          </thead>
          <tbody>
            {calls.map((call) => (
              <tr key={call.id}>
                <td className="primary-cell">{call.leadName}</td>
                <td>{call.phone}</td>
                <td>{call.campaign}</td>
                <td><span className={statusClass(call.status)}>{call.status}</span></td>
                <td><span className="chip">{call.outcome}</span></td>
                <td>{call.durationSec ? `${Math.round(call.durationSec / 60)} min` : "-"}</td>
                <td className="summary-cell">{call.summary || "Pendiente de ejecucion"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
