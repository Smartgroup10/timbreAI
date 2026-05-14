"use client";

import { useEffect, useState } from "react";
import { StatCard } from "../../../components/stat-card";
import { api, statusClass } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../lib/i18n";

type EndpointState = { resource: string; state: string; channel_ids: string[] };

export default function OperationsPage() {
  const { user } = useAuth();
  const t = useT();
  const statusLabel = useStatusLabel();
  const overview = useResource(() => api.overview(), []);
  const ops = useResource(() => api.operations(), []);
  const trunks = useResource(() => api.adminTrunks(), []);
  const tenants = useResource(() => api.tenants(), []);
  const audit = useResource(() => api.adminAudit(), []);

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
    return <div className="empty-state danger">{t("admin.tenants.access.denied")}</div>;
  }

  const ariEnabled = Boolean(ops.data?.ariEnabled);
  const voiceAgentReachable = Boolean(ops.data?.voiceAgentReachable);
  const voiceProviders = (ops.data?.voiceProviders as string[] | undefined) ?? [];
  const realProviders = voiceProviders.filter((p) => p !== "echo");
  const trunksData = trunks.data ?? [];
  const tenantsData = tenants.data ?? [];
  const auditData = (audit.data ?? []).slice(0, 12);

  const activeChannels = Object.values(sipState).reduce((acc, ep) => acc + (ep.channel_ids?.length ?? 0), 0);
  const registeredTrunks = Object.values(sipState).filter((ep) => ep.state === "online").length;

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("admin.eyebrow")}</p>
          <h1>{t("nav.operations")}</h1>
          <p className="subtle">{t("admin.ops.subtitle.full")}</p>
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
            {t("admin.ops.btn.refresh")}
          </button>
        </div>
      </div>

      <div className="grid">
        <StatCard
          label={t("admin.ops.stat.activecalls")}
          value={activeChannels}
          hint={t("admin.ops.stat.activecalls.hint")}
          trend={activeChannels > 0 ? t("admin.ops.stat.activecalls.trend") : ""}
        />
        <StatCard
          label={t("admin.ops.stat.trunks")}
          value={`${registeredTrunks}/${trunksData.length}`}
          hint={t("admin.ops.stat.trunks.hint")}
          trend={ariEnabled ? t("admin.ops.stat.trunks.ari.on") : t("admin.ops.stat.trunks.ari.off")}
        />
        <StatCard
          label={t("admin.ops.stat.tenants")}
          value={tenantsData.length}
          hint={t("admin.ops.stat.tenants.hint")}
        />
        <StatCard
          label={t("admin.ops.stat.voiceagent")}
          value={voiceAgentReachable ? "OK" : "Down"}
          hint={
            realProviders.length > 0
              ? t("admin.ops.stat.voiceagent.providers", { n: realProviders.length })
              : t("admin.ops.stat.voiceagent.echoonly")
          }
          trend={realProviders.length > 0 ? realProviders.join(", ") : ""}
        />
      </div>

      <div className="grid two" style={{ marginTop: 16 }}>
        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("admin.ops.health.eyebrow")}</p>
              <h2>{t("admin.ops.health.title")}</h2>
            </div>
          </div>
          <div className="command-strip">
            <Row label="Postgres" ok ok_msg={t("admin.ops.health.postgres")} />
            <Row label="Asterisk ARI" ok={ariEnabled} ok_msg={t("admin.ops.health.ari.ok")} ko_msg={t("admin.ops.health.ari.ko")} />
            <Row
              label="Voice agent"
              ok={voiceAgentReachable}
              ok_msg={t("admin.ops.health.voiceagent.ok", { list: voiceProviders.join(", ") })}
              ko_msg={t("admin.ops.health.voiceagent.ko")}
            />
            <Row
              label="Trunks SIP"
              ok={registeredTrunks > 0}
              ok_msg={t("admin.ops.health.trunks.ok", { ok: registeredTrunks, total: trunksData.length })}
              ko_msg={trunksData.length === 0 ? t("admin.ops.health.trunks.notrunks") : t("admin.ops.health.trunks.none")}
            />
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <p className="eyebrow">{t("admin.ops.trunks.eyebrow")}</p>
              <h2>{t("admin.ops.trunks.title")}</h2>
            </div>
            <a className="button ghost compact" href="/admin/trunks">{t("admin.ops.trunks.manage")}</a>
          </div>
          {trunksData.length === 0 ? (
            <p className="subtle">{t("admin.ops.trunks.empty")}</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>{t("admin.ops.trunks.col.endpoint")}</th>
                  <th>{t("admin.ops.trunks.col.provider")}</th>
                  <th>{t("admin.ops.trunks.col.state")}</th>
                  <th>{t("admin.ops.trunks.col.channels")}</th>
                </tr>
              </thead>
              <tbody>
                {trunksData.map((tr) => {
                  const sip = sipState[tr.asteriskEndpoint];
                  const stateLabel = sip
                    ? sip.state === "online"
                      ? t("admin.ops.trunks.state.registered")
                      : sip.state
                    : t("admin.ops.trunks.state.unknown");
                  return (
                    <tr key={tr.id}>
                      <td>
                        <code className="mono">{tr.asteriskEndpoint}</code>
                      </td>
                      <td>{tr.provider || "—"}</td>
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
            <p className="eyebrow">{t("admin.ops.audit.eyebrow")}</p>
            <h2>{t("admin.ops.audit.title")}</h2>
          </div>
          <a className="button ghost compact" href="/admin/audit">{t("admin.ops.audit.viewall")}</a>
        </div>
        {auditData.length === 0 ? (
          <p className="subtle">{t("admin.ops.audit.empty")}</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>{t("admin.ops.audit.col.when")}</th>
                <th>{t("admin.ops.audit.col.tenant")}</th>
                <th>{t("admin.ops.audit.col.actor")}</th>
                <th>{t("admin.ops.audit.col.action")}</th>
                <th>{t("admin.ops.audit.col.entity")}</th>
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
            <p className="eyebrow">{t("admin.ops.build.eyebrow")}</p>
            <h2>{t("admin.ops.build.title")}</h2>
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
  const t = useT();
  return (
    <div className="command-row">
      <span>{label}</span>
      <span className={ok ? "status good" : "status warn"}>{ok ? ok_msg || t("admin.ops.row.ok") : ko_msg || t("admin.ops.row.pending")}</span>
    </div>
  );
}
