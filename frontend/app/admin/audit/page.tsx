"use client";

import { AuditTable } from "../../../components/audit-table";
import { api } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT } from "../../../lib/i18n";

export default function AdminAuditPage() {
  const { user } = useAuth();
  const t = useT();
  const audit = useResource(() => api.adminAudit(), [], { pollMs: 20_000 });

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">{t("admin.tenants.access.denied")}</div>;
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("admin.eyebrow")}</p>
          <h1>{t("audit.global.title.full")}</h1>
          <p className="subtle">{t("audit.global.subtitle.full")}</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => audit.reload()}>
            {t("audit.btn.refresh")}
          </button>
        </div>
      </div>

      <AuditTable rows={audit.data ?? []} loading={audit.loading} error={audit.error} showTenant />
    </>
  );
}
