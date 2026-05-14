"use client";

import { useEffect, useState } from "react";
import { StatCard } from "../../../components/stat-card";
import { api, statusClass, statusLabel } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

type EndpointState = { resource: string; state: string; channel_ids: string[] };

export default function OperationsPage() {
  const { user } = useAuth();
  const overview = useResource(() => api.overview(), []);
  const ops = useResource(() => api.operations(), []);
  const trunks = useResource(() => api.adminTrunks(), []);
  const tenants = useResource(() => api.tenants(), []);
  const audit = useResource(() => api.adminAudit(), []);

  // Estado SIP en vivo de cada trunk vía ARI — actualiza cada 10s.
  const [sipState, setSipState] = useState<Record<string, EndpointState>>({});
  useEffect(() => {
    let cancelled = false;
    async function refresh() {
      try {
        const d = await api.adminTrunkStatus();
        if (cancelled) return;
        const map: Record<string, EndpointState> = {};
        for (const ep of d.endpoints) map[ep.resource] = ep;
        setSipState(map);
      } catch {
        /* ignore */
      }
    }
    refresh();
    const id = setInterval(refresh, 10_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">Acceso restringido al rol platform_admin.</div>;
  }

  const ariEnabled = Boolean(ops.data?.ariEnabled);
  const voiceAgentReachable = Boolean(ops.data?.voiceAgentReachable);
  const voiceProviders = (ops.data?.voiceProviders as string[] | undefined) ?? [];
  const realProviders = voiceProviders.filter((p) => p !== "echo");
  const trunksData = trunks.data ?? [];
  const tenantsData = tenants.data ?? [];
  const auditData = (audit.data ?? []).slice(0, 12);

  // Llamadas activas según ARI: sumamos los channel_ids de todos los endpoints.
  const activeChannels = Object.values(sipState).reduce((acc, ep) => acc + (ep.channel_ids?.length ?? 0), 0);
  const registeredTrunks = Object.values(sipState).filter((ep) => ep.state === "online").length;

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Operaciones</h1>
          <p className="subtle">Salud de servicios, trunks SIP, llamadas activas y actividad global de la plataforma.</p>
        </div>
        <div className="actions">
          <button
            className="button secondary"
            onClick={() => {
              overview.reload();
              ops.reload();
              trunks.reload();
              audit.reload();
            }}
          >
            Refrescar
          </button>
        </div>
      </div>

      <div className="grid">
        <StatCard
          label="Llamadas activas"
          value={activeChannels}
          hint="Channels en ARI ahora"
          trend={activeChannels > 0 ? "Live" : ""}
        />
        <StatCard
          label="Trunks registrados"
          value={`${registeredTrunks}/${trunksData.length}`}
          hint="Endpoints SIP online"
          trend={ariEnabled ? "ARI conectado" : "ARI off"}
        />
        <StatCard
          label="Tenants"
          value={tenantsData.length}
          hint="Total en plataforma"
        />
        <StatCard
          label="Voice agent"
          value={voiceAgentReachable ? "OK" : "Down"}
          hint={
            realProviders.length > 0
              ? `${realProviders.length} providers reales`
              : "Solo echo (sin API keys reales)"
          }
          trend={realProviders.length > 0 ? realProviders.join(", ") : ""}
        />
      </div>

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Salud de servicios</p>
              <h2>Cluster</h2>
            </div>
          </div>
          <div className="command-strip">
            <Row label="Postgres" ok ok_msg="Conectado (estás leyendo de él)" />
            <Row label="Asterisk ARI" ok={ariEnabled} ok_msg="WS conectado" ko_msg="Deshabilitado o sin handshake" />
            <Row label="Voice agent" ok={voiceAgentReachable} ok_msg={`Reachable · providers: ${voiceProviders.join(", ")}`} ko_msg="No responde al ping" />
            <Row
              label="Trunks SIP"
              ok={registeredTrunks > 0}
              ok_msg={`${registeredTrunks}/${trunksData.length} registrados al proveedor`}
              ko_msg={trunksData.length === 0 ? "No hay trunks configurados" : "Ninguno registrado — revisa creds"}
            />
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">Trunks</p>
              <h2>Estado SIP en vivo</h2>
            </div>
            <a className="button ghost compact" href="/admin/trunks">Gestionar</a>
          </div>
          {trunksData.length === 0 ? (
            <p className="subtle">Aún no hay trunks. Crea uno desde Trunks y DIDs.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Endpoint</th>
                  <th>Proveedor</th>
                  <th>Estado</th>
                  <th>Channels</th>
                </tr>
              </thead>
              <tbody>
                {trunksData.map((t) => {
                  const sip = sipState[t.asteriskEndpoint];
                  const stateLabel = sip ? (sip.state === "online" ? "registrado" : sip.state) : "desconocido";
                  return (
                    <tr key={t.id}>
                      <td>
                        <code className="mono">{t.asteriskEndpoint}</code>
                      </td>
                      <td>{t.provider || "—"}</td>
                      <td>
                        <span className={statusClass(stateLabel)}>{statusLabel(stateLabel)}</span>
                      </td>
                      <td>{sip?.channel_ids?.length ?? 0}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </section>
      </div>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Auditoría reciente</p>
            <h2>Últimos cambios</h2>
          </div>
          <a className="button ghost compact" href="/admin/audit">Ver todo</a>
        </div>
        {auditData.length === 0 ? (
          <p className="subtle">Sin actividad reciente.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Hora</th>
                <th>Tenant</th>
                <th>Actor</th>
                <th>Acción</th>
                <th>Entidad</th>
              </tr>
            </thead>
            <tbody>
              {auditData.map((a) => (
                <tr key={a.id}>
                  <td>
                    <code className="mono">{new Date(a.createdAt).toLocaleTimeString()}</code>
                  </td>
                  <td>{a.tenantId ?? "—"}</td>
                  <td>{a.actorEmail || a.actorId}</td>
                  <td>
                    <code className="mono">{a.action}</code>
                  </td>
                  <td>
                    <span className="subtle">
                      {a.entityType}/{a.entityId.slice(0, 16)}
                      {a.entityId.length > 16 ? "…" : ""}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Build info</p>
            <h2>Versión desplegada</h2>
          </div>
          <span className="chip">v{(ops.data?.version as string) || "?"}</span>
        </div>
        <div className="command-strip">
          <Row label="ARI App" ok ok_msg={(ops.data?.ariApp as string) || "—"} />
          <Row label="JWT TTL" ok ok_msg={`${ops.data?.jwtTtlHours ?? "?"} h`} />
          <Row label="SIP test extension" ok ok_msg={(ops.data?.sipTestExt as string) || "—"} />
        </div>
      </section>
    </>
  );
}

function Row({ label, ok, ok_msg, ko_msg }: { label: string; ok: boolean; ok_msg?: string; ko_msg?: string }) {
  return (
    <div className="command-row">
      <span>{label}</span>
      <span className={ok ? "status good" : "status warn"}>{ok ? ok_msg || "OK" : ko_msg || "Pendiente"}</span>
    </div>
  );
}
