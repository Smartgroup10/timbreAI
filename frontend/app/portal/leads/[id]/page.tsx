"use client";

import { use, useState } from "react";
import Link from "next/link";
import { ArrowLeft, PhoneCall, Trash2 } from "lucide-react";
import { TestCallDrawer } from "../../../../components/test-call-drawer";
import { useToast } from "../../../../components/toast";
import { api, ApiError, statusClass } from "../../../../lib/api";
import { useTenantScope } from "../../../../lib/auth-context";
import { useResource } from "../../../../lib/use-resource";

const STATUS_OPTIONS = ["new", "qualified", "callback", "contacted", "do_not_call"];

export default function LeadDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const tenant = useTenantScope();
  const lead = useResource(() => api.getLead(id, tenant), [id, tenant]);
  const calls = useResource(() => api.leadCalls(id, tenant), [id, tenant]);
  const toast = useToast();
  const [drawerOpen, setDrawerOpen] = useState(false);

  async function handleStatus(status: string) {
    try {
      await api.updateLead(id, { status }, tenant);
      toast.push("Estado actualizado", "success");
      lead.reload();
      calls.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleDelete() {
    if (!lead.data) return;
    if (!confirm(`Eliminar a "${lead.data.name}"?`)) return;
    try {
      await api.deleteLead(id, tenant);
      toast.push("Lead eliminado", "success");
      window.location.href = "/portal/leads";
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  if (lead.loading) return <div className="empty-state">Cargando lead…</div>;
  if (lead.error) return <div className="empty-state danger">Error: {lead.error}</div>;
  if (!lead.data) return <div className="empty-state">Lead no encontrado.</div>;

  const l = lead.data;
  const callList = calls.data ?? [];
  const totalDuration = callList.reduce((acc, c) => acc + c.durationSec, 0);

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
          <Link href="/portal/leads" className="button ghost compact" style={{ marginBottom: 8 }}>
            <ArrowLeft aria-hidden="true" />
            <span>Volver a leads</span>
          </Link>
          <p className="eyebrow">Lead</p>
          <h1>{l.name}</h1>
          <p className="subtle">
            <code className="mono">{l.phone}</code> · {l.email || "sin email"}
          </p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setDrawerOpen(true)}>
            <PhoneCall aria-hidden="true" />
            <span>Llamada de prueba</span>
          </button>
          <button className="button ghost" onClick={handleDelete}>
            <Trash2 aria-hidden="true" />
            <span>Eliminar</span>
          </button>
        </div>
      </div>

      <div className="grid two">
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Datos</p>
              <h2>Información del lead</h2>
            </div>
            <span className={statusClass(l.status)}>{l.status}</span>
          </div>
          <div className="command-strip">
            <Row label="Tipo" value={<span className="chip">{l.type}</span>} />
            <Row label="Fuente" value={l.source} />
            <Row label="Consentimiento" value={l.consent} />
            <Row label="Última actividad" value={new Date(l.lastActivity).toLocaleString()} />
          </div>
          <div className="field" style={{ marginTop: 14 }}>
            <label>Cambiar estado</label>
            <select value={l.status} onChange={(e) => handleStatus(e.target.value)}>
              {STATUS_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Resumen</p>
              <h2>Actividad</h2>
            </div>
          </div>
          <div className="command-strip">
            <Row label="Total llamadas" value={<strong>{callList.length}</strong>} />
            <Row label="Tiempo total" value={formatDuration(totalDuration)} />
            <Row
              label="Última llamada"
              value={
                callList[0]?.startedAt
                  ? new Date(callList[0].startedAt).toLocaleString()
                  : <span className="subtle">Nunca</span>
              }
            />
            <Row
              label="Outcome dominante"
              value={
                callList.length === 0 ? (
                  <span className="subtle">—</span>
                ) : (
                  <span className="chip">
                    {topOutcome(callList.map((c) => c.outcome))}
                  </span>
                )
              }
            />
          </div>
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Historial</p>
            <h2>Llamadas ({callList.length})</h2>
          </div>
        </div>
        {calls.loading ? (
          <div className="empty-state">Cargando…</div>
        ) : callList.length === 0 ? (
          <div className="empty-state">Aún no hay llamadas para este lead.</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Fecha</th>
                  <th>Campaña</th>
                  <th>Estado</th>
                  <th>Outcome</th>
                  <th>Duración</th>
                  <th>Resumen</th>
                </tr>
              </thead>
              <tbody>
                {callList.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <Link href={`/portal/calls/${c.id}`} style={{ color: "inherit" }}>
                        {c.startedAt ? new Date(c.startedAt).toLocaleString() : "—"}
                      </Link>
                    </td>
                    <td>{c.campaign || "Manual"}</td>
                    <td>
                      <span className={statusClass(c.status)}>{c.status}</span>
                    </td>
                    <td>
                      <span className="chip">{c.outcome}</span>
                    </td>
                    <td>{formatDuration(c.durationSec)}</td>
                    <td className="summary-cell">
                      <Link href={`/portal/calls/${c.id}`} style={{ color: "inherit" }}>
                        {c.summary || "Ver detalle"}
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <TestCallDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        defaultPhone={l.phone}
        defaultLeadName={l.name}
        onCallCreated={() => calls.reload()}
      />
    </>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="command-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function topOutcome(outcomes: string[]): string {
  const tally: Record<string, number> = {};
  for (const o of outcomes) tally[o] = (tally[o] || 0) + 1;
  let best = "";
  let bestCount = -1;
  for (const [k, v] of Object.entries(tally)) {
    if (v > bestCount) {
      best = k;
      bestCount = v;
    }
  }
  return best || "—";
}
