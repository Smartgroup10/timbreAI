"use client";

import { AuditTable } from "../../../components/audit-table";
import { api } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

export default function TenantAuditPage() {
  const tenant = useTenantScope();
  const audit = useResource(() => api.audit(tenant), [tenant]);

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Actividad reciente</h1>
          <p className="subtle">
            Quién hizo qué dentro de tu tenant. Útil para auditoría interna, debugging y para entender por qué un
            bot, lead o campaña cambió de estado.
          </p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => audit.reload()}>
            Refrescar
          </button>
        </div>
      </div>

      <AuditTable rows={audit.data ?? []} loading={audit.loading} error={audit.error} />
    </>
  );
}
