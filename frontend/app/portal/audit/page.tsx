"use client";

import { AuditTable } from "../../../components/audit-table";
import { api } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT } from "../../../lib/i18n";

export default function TenantAuditPage() {
  const tenant = useTenantScope();
  const t = useT();
  const audit = useResource(() => api.audit(tenant), [tenant]);

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("audit.tenant.title")}</h1>
          <p className="subtle">{t("audit.tenant.subtitle")}</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => audit.reload()}>
            {t("audit.btn.refresh")}
          </button>
        </div>
      </div>

      <AuditTable rows={audit.data ?? []} loading={audit.loading} error={audit.error} />
    </>
  );
}
