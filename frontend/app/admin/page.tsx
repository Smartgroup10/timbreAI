import { api, statusClass } from "../../lib/api";

export default async function AdminPage() {
  const tenants = await api.tenants();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Clientes</h1>
          <p className="subtle">Gestion multi-tenant de cuentas, limites, estado operativo y soporte.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Auditoria</button>
          <button className="button">Crear cliente</button>
        </div>
      </div>

      <div className="grid three" style={{ marginBottom: 16 }}>
        <section className="panel">
          <p className="eyebrow">Tenants</p>
          <span className="stat-value">{tenants.length}</span>
          <p className="subtle">Cuentas visibles para admin plataforma.</p>
        </section>
        <section className="panel">
          <p className="eyebrow">Telefonia</p>
          <span className="stat-value">1</span>
          <p className="subtle">Asterisk configurado en modo sandbox.</p>
        </section>
        <section className="panel">
          <p className="eyebrow">Riesgo</p>
          <span className="stat-value">0</span>
          <p className="subtle">Sin opt-outs pendientes en datos demo.</p>
        </section>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Cliente</th>
              <th>Tenant</th>
              <th>Plan</th>
              <th>Estado</th>
              <th>Creado</th>
            </tr>
          </thead>
          <tbody>
            {tenants.map((tenant) => (
              <tr key={tenant.id}>
                <td className="primary-cell">{tenant.name}</td>
                <td>{tenant.id}</td>
                <td><span className="chip">{tenant.plan}</span></td>
                <td><span className={statusClass(tenant.status)}>{tenant.status}</span></td>
                <td>{new Date(tenant.createdAt).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
