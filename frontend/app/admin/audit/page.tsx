"use client";

import { AuditTable } from "../../../components/audit-table";
import { api } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

export default function AdminAuditPage() {
  const { user } = useAuth();
  const audit = useResource(() => api.adminAudit(), []);

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">Acceso restringido al rol platform_admin.</div>;
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Audit log global</h1>
          <p className="subtle">
            Eventos de toda la plataforma con tenant, actor, acción y payload. Indexado por <code>created_at</code>
            descendente, últimos 200 eventos.
          </p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => audit.reload()}>
            Refrescar
          </button>
        </div>
      </div>

      <AuditTable rows={audit.data ?? []} loading={audit.loading} error={audit.error} showTenant />
    </>
  );
}
