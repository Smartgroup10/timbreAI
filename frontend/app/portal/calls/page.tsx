import { api, statusClass } from "../../../lib/api";

export default async function CallsPage() {
  const calls = await api.calls();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Llamadas</h1>
          <p className="subtle">Historial con resultado, duracion y resumen del bot.</p>
        </div>
        <button className="button secondary">Llamada de prueba</button>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Lead</th>
              <th>Telefono</th>
              <th>Campaña</th>
              <th>Estado</th>
              <th>Resultado</th>
              <th>Duracion</th>
              <th>Resumen</th>
            </tr>
          </thead>
          <tbody>
            {calls.map((call) => (
              <tr key={call.id}>
                <td>{call.leadName}</td>
                <td>{call.phone}</td>
                <td>{call.campaign}</td>
                <td><span className={statusClass(call.status)}>{call.status}</span></td>
                <td>{call.outcome}</td>
                <td>{call.durationSec ? `${Math.round(call.durationSec / 60)} min` : "-"}</td>
                <td>{call.summary || "Pendiente"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

