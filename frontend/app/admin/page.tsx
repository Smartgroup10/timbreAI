import { api, statusClass } from "../../lib/api";

export default async function AdminPage() {
  const tenants = await api.tenants();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Admin interno</p>
          <h1>Clientes</h1>
          <p className="subtle">Gestion multi-tenant de cuentas, limites y estado operativo.</p>
        </div>
        <button className="button">Crear cliente</button>
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
                <td>{tenant.name}</td>
                <td>{tenant.id}</td>
                <td>{tenant.plan}</td>
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

