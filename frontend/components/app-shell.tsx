import Link from "next/link";

const portalLinks = [
  ["DB", "Dashboard", "/portal"],
  ["LD", "Leads", "/portal/leads"],
  ["PR", "Propiedades", "/portal/properties"],
  ["BT", "Bots", "/portal/bots"],
  ["CP", "Campanas", "/portal/campaigns"],
  ["LL", "Llamadas", "/portal/calls"],
  ["ST", "Configuracion", "/portal/settings"]
];

const adminLinks = [
  ["CL", "Clientes", "/admin"],
  ["OP", "Operaciones", "/admin/operations"]
];

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">AC</div>
          <div>
            <strong>Atrium Calls</strong>
            <span>Voice AI operations</span>
          </div>
        </div>
        <div className="tenant-card">
          <span>Tenant activo</span>
          <strong>Atrium Leasing</strong>
          <span>Sandbox operativo</span>
        </div>
        <div className="nav-section">Portal cliente</div>
        <div className="nav-group">
          {portalLinks.map(([code, label, href]) => (
            <Link className="nav-link" href={href} key={href}>
              <span className="nav-dot">{code}</span>
              <span>{label}</span>
            </Link>
          ))}
        </div>
        <div className="nav-section">Admin interno</div>
        <div className="nav-group">
          {adminLinks.map(([code, label, href]) => (
            <Link className="nav-link" href={href} key={href}>
              <span className="nav-dot">{code}</span>
              <span>{label}</span>
            </Link>
          ))}
        </div>
        <div className="nav-meta">
          ARI preparado. Postgres y Redis definidos. Llamadas reales pendientes de trunk y worker.
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
