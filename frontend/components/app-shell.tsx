import Link from "next/link";

const portalLinks = [
  ["Dashboard", "/portal"],
  ["Leads", "/portal/leads"],
  ["Propiedades", "/portal/properties"],
  ["Bots", "/portal/bots"],
  ["Campañas", "/portal/campaigns"],
  ["Llamadas", "/portal/calls"],
  ["Configuracion", "/portal/settings"]
];

const adminLinks = [
  ["Clientes", "/admin"],
  ["Operaciones", "/admin/operations"]
];

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <strong>Atrium Calls</strong>
          <span>Voice AI operations</span>
        </div>
        <div className="nav-section">Portal cliente</div>
        {portalLinks.map(([label, href]) => (
          <Link className="nav-link" href={href} key={href}>
            {label}
          </Link>
        ))}
        <div className="nav-section">Admin interno</div>
        {adminLinks.map(([label, href]) => (
          <Link className="nav-link" href={href} key={href}>
            {label}
          </Link>
        ))}
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}

