"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Activity,
  Bot,
  Building2,
  ClipboardList,
  Home,
  LayoutDashboard,
  LogOut,
  Megaphone,
  PhoneCall,
  PhoneOff,
  Server,
  Settings,
  ShieldAlert,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { api, Tenant } from "../lib/api";
import { useAuth } from "../lib/auth-context";

type NavItem = { icon: LucideIcon; label: string; href: string };

const portalLinks: NavItem[] = [
  { icon: LayoutDashboard, label: "Dashboard", href: "/portal" },
  { icon: Users, label: "Leads", href: "/portal/leads" },
  { icon: Home, label: "Propiedades", href: "/portal/properties" },
  { icon: Bot, label: "Bots", href: "/portal/bots" },
  { icon: Megaphone, label: "Campañas", href: "/portal/campaigns" },
  { icon: PhoneCall, label: "Llamadas", href: "/portal/calls" },
  { icon: PhoneOff, label: "Do Not Call", href: "/portal/do-not-call" },
  { icon: ClipboardList, label: "Auditoría", href: "/portal/audit" },
  { icon: Settings, label: "Configuración", href: "/portal/settings" },
];

const adminLinks: NavItem[] = [
  { icon: Building2, label: "Clientes", href: "/admin" },
  { icon: Server, label: "Trunks y DIDs", href: "/admin/trunks" },
  { icon: ShieldAlert, label: "Audit global", href: "/admin/audit" },
  { icon: Activity, label: "Operaciones", href: "/admin/operations" },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname() || "";
  const { user, logout, tenantOverride, setTenantOverride } = useAuth();
  const isAdmin = user?.role === "platform_admin";
  const [tenants, setTenants] = useState<Tenant[]>([]);

  useEffect(() => {
    if (!isAdmin) return;
    api
      .tenants()
      .then(setTenants)
      .catch(() => setTenants([]));
  }, [isAdmin]);

  const activeTenantId = tenantOverride || user?.tenantId || (tenants[0]?.id ?? "");
  const activeTenantName = tenants.find((t) => t.id === activeTenantId)?.name || activeTenantId || "Tenant";

  function isActive(href: string) {
    if (href === "/portal" || href === "/admin") return pathname === href;
    return pathname === href || pathname.startsWith(href + "/");
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">CH</div>
          <div>
            <strong>CallHub</strong>
            <span>Voice AI operations</span>
          </div>
        </div>

        <div className="tenant-card">
          <span>Tenant activo</span>
          <strong>{activeTenantName || "—"}</strong>
          {isAdmin ? (
            <select
              className="tenant-select"
              value={tenantOverride}
              onChange={(event) => setTenantOverride(event.target.value)}
            >
              <option value="">Mi tenant (si lo tengo)</option>
              {tenants.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </select>
          ) : (
            <span>{user?.role === "tenant_admin" ? "Acceso operador" : "Acceso usuario"}</span>
          )}
        </div>

        <div className="nav-section">Portal cliente</div>
        <div className="nav-group">
          {portalLinks.map(({ icon: Icon, label, href }) => (
            <Link
              className={`nav-link${isActive(href) ? " active" : ""}`}
              href={href}
              key={href}
            >
              <Icon aria-hidden="true" />
              <span>{label}</span>
            </Link>
          ))}
        </div>

        {isAdmin ? (
          <>
            <div className="nav-section">Admin interno</div>
            <div className="nav-group">
              {adminLinks.map(({ icon: Icon, label, href }) => (
                <Link
                  className={`nav-link${isActive(href) ? " active" : ""}`}
                  href={href}
                  key={href}
                >
                  <Icon aria-hidden="true" />
                  <span>{label}</span>
                </Link>
              ))}
            </div>
          </>
        ) : null}

        <div className="user-card">
          <div className="user-card-row">
            <span className="user-avatar">{(user?.name || user?.email || "?").slice(0, 1).toUpperCase()}</span>
            <div className="user-meta">
              <strong>{user?.name || user?.email}</strong>
              <span>{user?.role}</span>
            </div>
          </div>
          <button className="button ghost compact logout-button" onClick={logout}>
            <LogOut aria-hidden="true" />
            <span>Cerrar sesión</span>
          </button>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
