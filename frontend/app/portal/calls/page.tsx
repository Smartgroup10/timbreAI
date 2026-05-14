"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { TestCallDrawer } from "../../../components/test-call-drawer";
import { api, statusClass, statusLabel } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

const STATUS_FILTERS = ["all", "completed", "queued", "dialing", "failed", "skipped"] as const;

export default function CallsPage() {
  const tenant = useTenantScope();
  const calls = useResource(() => api.calls(tenant), [tenant]);
  const [filter, setFilter] = useState<(typeof STATUS_FILTERS)[number]>("all");
  const [drawerOpen, setDrawerOpen] = useState(false);

  const filtered = useMemo(() => {
    const data = calls.data ?? [];
    if (filter === "all") return data;
    return data.filter((c) => c.status === filter);
  }, [calls.data, filter]);

  function formatDuration(sec: number) {
    if (!sec) return "—";
    if (sec < 60) return `${sec}s`;
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return s === 0 ? `${m}m` : `${m}m ${s}s`;
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Llamadas</h1>
          <p className="subtle">Historial operativo con resultado, duración y resumen accionable del bot.</p>
        </div>
        <div className="actions">
          <button className="button secondary" disabled>
            Exportar
          </button>
          <button className="button" onClick={() => setDrawerOpen(true)}>
            Llamada de prueba
          </button>
        </div>
      </div>

      <div className="filter-row">
        {STATUS_FILTERS.map((opt) => (
          <button
            key={opt}
            className={`chip-button${filter === opt ? " active" : ""}`}
            onClick={() => setFilter(opt)}
          >
            {opt === "all" ? "Todas" : statusLabel(opt)}
          </button>
        ))}
        <button className="chip-button" onClick={() => calls.reload()}>
          Refrescar
        </button>
      </div>

      <div className="table-wrap">
        {calls.loading ? (
          <div className="empty-state">Cargando…</div>
        ) : calls.error ? (
          <div className="empty-state danger">Error: {calls.error}</div>
        ) : filtered.length === 0 ? (
          <div className="empty-state">Sin llamadas que coincidan con el filtro.</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Lead</th>
                <th>Teléfono</th>
                <th>Campaña</th>
                <th>Estado</th>
                <th>Resultado</th>
                <th>Duración</th>
                <th>Canal</th>
                <th>Resumen</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((call) => (
                <tr key={call.id} style={{ cursor: "pointer" }}>
                  <td className="primary-cell">
                    <Link href={`/portal/calls/${call.id}`} style={{ color: "inherit" }}>
                      {call.leadName || "—"}
                    </Link>
                  </td>
                  <td>{call.phone}</td>
                  <td>{call.campaign || "—"}</td>
                  <td>
                    <span className={statusClass(call.status)}>{statusLabel(call.status)}</span>
                  </td>
                  <td>
                    <span className="chip">{statusLabel(call.outcome)}</span>
                  </td>
                  <td>{formatDuration(call.durationSec)}</td>
                  <td>
                    <code className="mono">{call.channelId || "—"}</code>
                  </td>
                  <td className="summary-cell">
                    <Link href={`/portal/calls/${call.id}`} style={{ color: "inherit" }}>
                      {call.summary || "Ver detalle"}
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <TestCallDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onCallCreated={() => calls.reload()}
      />
    </>
  );
}
