"use client";

import { AuditLogEntry } from "../lib/api";
import { useT } from "../lib/i18n";

export function AuditTable({ rows, loading, error, showTenant = false }: {
  rows: AuditLogEntry[];
  loading: boolean;
  error: string | null;
  showTenant?: boolean;
}) {
  const t = useT();

  function actionLabel(action: string): string {
    const key = `audit.action.${action}`;
    const translated = t(key);
    return translated === key ? action : translated;
  }

  if (loading) return <div className="empty-state">{t("audit.table.loading")}</div>;
  if (error) return <div className="empty-state danger">{t("g.error")}: {error}</div>;
  if (rows.length === 0) return <div className="empty-state">{t("audit.table.empty")}</div>;

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t("audit.col.when")}</th>
            <th>{t("audit.col.action")}</th>
            {showTenant ? <th>{t("audit.col.tenant")}</th> : null}
            <th>{t("audit.col.actor")}</th>
            <th>{t("audit.col.entity")}</th>
            <th>{t("audit.col.details")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id}>
              <td><time>{new Date(row.createdAt).toLocaleString()}</time></td>
              <td>
                <span className="chip">{actionLabel(row.action)}</span>
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
