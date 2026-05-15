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
  Mic,
  PhoneCall,
  PhoneOff,
  Server,
  Settings,
  ShieldAlert,
  Users,
  Wrench,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { api, Tenant } from "../lib/api";
import { useAuth } from "../lib/auth-context";
import { useT, useLang } from "../lib/i18n";
import { BrandMark } from "./logo";

type NavItem = { icon: LucideIcon; labelKey: string; href: string };

const portalLinks: NavItem[] = [
  { icon: LayoutDashboard, labelKey: "nav.dashboard", href: "/portal" },
  { icon: Users, labelKey: "nav.leads", href: "/portal/leads" },
  { icon: Home, labelKey: "nav.properties", href: "/portal/properties" },
  { icon: Bot, labelKey: "nav.bots", href: "/portal/bots" },
  { icon: Wrench, labelKey: "nav.tools", href: "/portal/tools" },
  { icon: Megaphone, labelKey: "nav.campaigns", href: "/portal/campaigns" },
  { icon: PhoneCall, labelKey: "nav.calls", href: "/portal/calls" },
  { icon: Mic, labelKey: "nav.recordings", href: "/portal/recordings" },
  { icon: PhoneOff, labelKey: "nav.dnc", href: "/portal/do-not-call" },
  { icon: ClipboardList, labelKey: "nav.audit", href: "/portal/audit" },
  { icon: Settings, labelKey: "nav.settings", href: "/portal/settings" },
];

const adminLinks: NavItem[] = [
  { icon: Building2, labelKey: "nav.tenants", href: "/admin" },
  { icon: Server, labelKey: "nav.trunks", href: "/admin/trunks" },
  { icon: ShieldAlert, labelKey: "nav.audit.global", href: "/admin/audit" },
  { icon: Activity, labelKey: "nav.operations", href: "/admin/operations" },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname() || "";
  const { user, logout, tenantOverride, setTenantOverride } = useAuth();
  const t = useT();
  const { lang, setLang } = useLang();
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
          <div className="brand-row">
            <BrandMark size={28} className="brand-mark" />
            <strong className="brand-name">
              timbre<span className="brand-ai">.ai</span>
            </strong>
          </div>
          <span className="brand-tagline">{t("shell.tagline")}</span>
        </div>

        <div className="tenant-card">
          <span>{t("shell.tenant.active")}</span>
          <strong>{activeTenantName || "—"}</strong>
          {isAdmin ? (
            <select
              className="tenant-select"
              value={tenantOverride}
              onChange={(event) => setTenantOverride(event.target.value)}
            >
              <option value="">{t("shell.tenant.mine")}</option>
              {tenants.map((tn) => (
                <option key={tn.id} value={tn.id}>
                  {tn.name}
                </option>
              ))}
            </select>
          ) : (
            <span>{user?.role === "tenant_admin" ? t("shell.access.operator") : t("shell.access.user")}</span>
          )}
        </div>

        <div className="nav-section">{t("shell.section.portal")}</div>
        <div className="nav-group">
          {portalLinks.map(({ icon: Icon, labelKey, href }) => (
            <Link
              className={`nav-link${isActive(href) ? " active" : ""}`}
              href={href}
              key={href}
            >
              <Icon aria-hidden="true" />
              <span>{t(labelKey)}</span>
            </Link>
          ))}
        </div>

        {isAdmin ? (
          <>
            <div className="nav-section">{t("shell.section.admin")}</div>
            <div className="nav-group">
              {adminLinks.map(({ icon: Icon, labelKey, href }) => (
                <Link
                  className={`nav-link${isActive(href) ? " active" : ""}`}
                  href={href}
                  key={href}
                >
                  <Icon aria-hidden="true" />
                  <span>{t(labelKey)}</span>
                </Link>
              ))}
            </div>
          </>
        ) : null}

        <div className="lang-switch">
          <span className="lang-switch-label">{t("lang.label")}</span>
          <div className="lang-switch-buttons">
            <button
              type="button"
              className={`lang-switch-btn${lang === "es" ? " active" : ""}`}
              onClick={() => setLang("es")}
              aria-pressed={lang === "es"}
            >
              ES
            </button>
            <button
              type="button"
              className={`lang-switch-btn${lang === "en" ? " active" : ""}`}
              onClick={() => setLang("en")}
              aria-pressed={lang === "en"}
            >
              EN
            </button>
          </div>
        </div>

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
            <span>{t("shell.logout")}</span>
          </button>
        </div>
      </aside>
      <main className="main">
        {isAdmin && tenantOverride ? (
          <div className="impersonation-banner" role="status">
            <span>
              {t("shell.impersonation.text", { tenant: activeTenantName })}
            </span>
            <button className="button ghost compact" onClick={() => setTenantOverride("")}>
              {t("shell.impersonation.exit")}
            </button>
          </div>
        ) : null}
        {children}
      </main>
    </div>
  );
}
