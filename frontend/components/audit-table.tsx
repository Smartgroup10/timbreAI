"use client";

import { AuditLogEntry } from "../lib/api";

const actionLabels: Record<string, string> = {
  "auth.login": "Login",
  "bot.create": "Bot creado",
  "bot.update": "Bot actualizado",
  "bot.delete": "Bot eliminado",
  "bot.assign_did": "DID asignado al bot",
  "campaign.update": "Campaña actualizada",
  "campaign.delete": "Campaña eliminada",
  "call.test_originate": "Llamada de prueba",
  "did.assign_tenant": "DID asignado a tenant",
  "dnc.add": "Número bloqueado (DNC)",
  "dnc.remove": "Número liberado",
  "lead.update": "Lead actualizado",
  "lead.delete": "Lead eliminado",
  "property.create": "Propiedad creada",
  "property.update": "Propiedad actualizada",
  "property.delete": "Propiedad eliminada",
  "user.password_change": "Cambio de contraseña",
};

export function AuditTable({ rows, loading, error, showTenant = false }: {
  rows: AuditLogEntry[];
  loading: boolean;
  error: string | null;
  showTenant?: boolean;
}) {
  if (loading) return <div className="empty-state">Cargando…</div>;
  if (error) return <div className="empty-state danger">Error: {error}</div>;
  if (rows.length === 0) return <div className="empty-state">Sin eventos registrados.</div>;

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Cuándo</th>
            <th>Acción</th>
            {showTenant ? <th>Tenant</th> : null}
            <th>Actor</th>
            <th>Entidad</th>
            <th>Detalles</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id}>
              <td><time>{new Date(row.createdAt).toLocaleString()}</time></td>
              <td>
                <span className="chip">{actionLabels[row.action] || row.action}</span>
              </td>
              {showTenant ? (
                <td>
                  <code className="mono">{row.tenantId || "—"}</code>
                </td>
              ) : null}
              <td>
                <span className="primary-cell">{row.actorEmail || row.actorId || "—"}</span>
              </td>
              <td>
                <code className="mono">{row.entityType}/{row.entityId}</code>
              </td>
              <td className="summary-cell">
                {row.payload && Object.keys(row.payload).length > 0 ? (
                  <code className="mono" style={{ fontSize: 11 }}>{JSON.stringify(row.payload)}</code>
                ) : (
                  <span className="subtle">—</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
