"use client";

import { use } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { api, statusClass, statusLabel } from "../../../../lib/api";
import { useTenantScope } from "../../../../lib/auth-context";
import { useResource } from "../../../../lib/use-resource";

export default function CallDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const tenant = useTenantScope();
  const call = useResource(() => api.getCall(id, tenant), [id, tenant]);
  const transcripts = useResource(() => api.callTranscripts(id, tenant), [id, tenant]);

  if (call.loading) {
    return <div className="empty-state">Cargando llamada…</div>;
  }
  if (call.error) {
    return <div className="empty-state danger">Error: {call.error}</div>;
  }
  if (!call.data) {
    return <div className="empty-state">Llamada no encontrada.</div>;
  }

  const c = call.data;
  const lines = transcripts.data ?? [];

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
          <Link href="/portal/calls" className="button ghost compact" style={{ marginBottom: 8 }}>
            <ArrowLeft aria-hidden="true" />
            <span>Volver a llamadas</span>
          </Link>
          <p className="eyebrow">Llamada</p>
          <h1>{c.leadName || c.phone}</h1>
          <p className="subtle">
            <code className="mono">{c.id}</code>
          </p>
        </div>
        <div className="actions">
          <span className={statusClass(c.status)}>{statusLabel(c.status)}</span>
          <span className="chip">{c.outcome}</span>
        </div>
      </div>

      <div className="grid two">
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Detalles</p>
              <h2>Datos de la llamada</h2>
            </div>
          </div>
          <div className="command-strip">
            <Row label="Teléfono" value={<code className="mono">{c.phone}</code>} />
            <Row label="Lead" value={c.leadName || "—"} />
            <Row label="Campaña" value={c.campaign || "—"} />
            <Row label="Duración" value={formatDuration(c.durationSec)} />
            <Row label="Inicio" value={c.startedAt ? new Date(c.startedAt).toLocaleString() : "—"} />
            <Row label="Fin" value={c.endedAt ? new Date(c.endedAt).toLocaleString() : "—"} />
            <Row label="Canal ARI" value={<code className="mono">{c.channelId || "—"}</code>} />
            <Row
              label="Voice session"
              value={c.voiceSessionId ? <code className="mono">{c.voiceSessionId}</code> : <span className="subtle">Sin sesión</span>}
            />
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Resumen</p>
              <h2>Outcome</h2>
            </div>
          </div>
          <p className="subtle">{c.summary || "El bot todavía no generó un resumen para esta llamada."}</p>

          {c.recordingUrl ? (
            <div style={{ marginTop: 14 }}>
              <p className="eyebrow">Grabación</p>
              <audio controls src={c.recordingUrl} style={{ width: "100%" }} />
            </div>
          ) : null}
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Conversación</p>
            <h2>Transcript ({lines.length})</h2>
          </div>
          <button className="button secondary compact" onClick={() => transcripts.reload()}>
            Refrescar
          </button>
        </div>
        {transcripts.loading ? (
          <div className="empty-state">Cargando…</div>
        ) : lines.length === 0 ? (
          <div className="empty-state">
            Sin transcripts persistidos todavía. El voice-agent los escribe via webhook cuando hay una sesión activa.
          </div>
        ) : (
          <div className="transcript">
            {lines.map((line) => (
              <div key={line.id} className={`transcript-line transcript-${line.role}`}>
                <span className="transcript-role">{line.role}</span>
                <span className="transcript-text">{line.text}</span>
                <time className="transcript-time">{new Date(line.occurredAt).toLocaleTimeString()}</time>
              </div>
            ))}
          </div>
        )}
      </section>
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
