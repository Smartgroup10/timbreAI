"use client";

import { useEffect, useState } from "react";
import { useToast } from "../../../components/toast";
import { api, ApiError, DID, SIPTrunk, Tenant, statusClass } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT, useStatusLabel } from "../../../lib/i18n";

type Tab = "trunks" | "dids";

type EndpointState = { state: string; channels: number };

export default function TrunksPage() {
  const { user } = useAuth();
  const t = useT();
  const statusLabel = useStatusLabel();
  const trunks = useResource(() => api.adminTrunks(), []);
  const dids = useResource(() => api.adminDIDs(), []);
  const tenants = useResource(() => api.tenants(), []);
  const [tab, setTab] = useState<Tab>("trunks");
  const [trunkFormOpen, setTrunkFormOpen] = useState(false);
  const [editingTrunk, setEditingTrunk] = useState<SIPTrunk | null>(null);
  const [didFormOpen, setDidFormOpen] = useState(false);
  const [editingDid, setEditingDid] = useState<DID | null>(null);
  const [sipState, setSipState] = useState<Record<string, EndpointState>>({});
  const [ariEnabled, setAriEnabled] = useState<boolean | null>(null);
  const toast = useToast();

  useEffect(() => {
    if (tab !== "trunks") return;
    let cancelled = false;
    async function refresh() {
      try {
        const data = await api.adminTrunkStatus();
        if (cancelled) return;
        setAriEnabled(data.ariEnabled);
        const map: Record<string, EndpointState> = {};
        for (const ep of data.endpoints) {
          map[ep.resource] = { state: ep.state, channels: ep.channel_ids?.length ?? 0 };
        }
        setSipState(map);
      } catch {
        if (!cancelled) setAriEnabled(false);
      }
    }
    refresh();
    const id = setInterval(refresh, 10_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [tab]);

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">{t("admin.tenants.access.denied")}</div>;
  }

  function reloadAll() {
    trunks.reload();
    dids.reload();
  }

  function openTrunkForm(editing?: SIPTrunk) {
    setEditingTrunk(editing ?? null);
    setTrunkFormOpen(true);
  }
  function closeTrunkForm() {
    setTrunkFormOpen(false);
    setEditingTrunk(null);
  }

  async function handleSaveTrunk(input: Partial<SIPTrunk>) {
    try {
      if (editingTrunk) {
        await api.adminUpdateTrunk(editingTrunk.id, input);
        toast.push(t("admin.trunks.toast.updated"), "success");
      } else {
        await api.adminCreateTrunk(input);
        toast.push(t("admin.trunks.toast.created"), "success");
      }
      closeTrunkForm();
      reloadAll();
    } catch (err) {
      toast.push(t("admin.trunks.toast.save_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleDeleteTrunk(id: string) {
    if (!confirm(t("admin.trunks.toast.delete_confirm"))) return;
    try {
      await api.adminDeleteTrunk(id);
      toast.push(t("admin.trunks.toast.deleted"), "success");
      reloadAll();
    } catch (err) {
      toast.push(t("admin.trunks.toast.delete_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  function openDidForm(editing?: DID) {
    setEditingDid(editing ?? null);
    setDidFormOpen(true);
  }
  function closeDidForm() {
    setDidFormOpen(false);
    setEditingDid(null);
  }

  async function handleSaveDID(input: Parameters<typeof api.adminCreateDID>[0]) {
    try {
      if (editingDid) {
        await api.adminUpdateDID(editingDid.id, {
          e164: input.e164,
          label: input.label,
          status: input.status,
        });
        toast.push(t("admin.trunks.dids.toast.updated"), "success");
      } else {
        await api.adminCreateDID(input);
        toast.push(t("admin.trunks.dids.toast.created"), "success");
      }
      closeDidForm();
      reloadAll();
    } catch (err) {
      toast.push(t("admin.trunks.toast.save_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleAssignDID(id: string, tenantId: string | null) {
    try {
      await api.adminAssignDID(id, tenantId);
      toast.push(tenantId ? t("admin.trunks.dids.toast.assigned") : t("admin.trunks.dids.toast.released"), "success");
      dids.reload();
    } catch (err) {
      toast.push(t("admin.trunks.dids.toast.assign_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleDeleteDID(id: string) {
    if (!confirm(t("admin.trunks.dids.toast.delete_confirm"))) return;
    try {
      await api.adminDeleteDID(id);
      toast.push(t("admin.trunks.dids.toast.deleted"), "success");
      reloadAll();
    } catch (err) {
      toast.push(t("admin.trunks.toast.delete_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  const trunksData = trunks.data ?? [];
  const didsData = dids.data ?? [];
  const tenantsData = tenants.data ?? [];

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("admin.eyebrow")}</p>
          <h1>{t("nav.trunks")}</h1>
          <p className="subtle">{t("admin.trunks.subtitle.full")}</p>
        </div>
        <div className="actions">
          {tab === "trunks" ? (
            <button className="button" onClick={() => (trunkFormOpen ? closeTrunkForm() : openTrunkForm())}>
              {trunkFormOpen ? t("admin.trunks.btn.cancel") : t("admin.trunks.btn.newtrunk")}
            </button>
          ) : (
            <button className="button" onClick={() => (didFormOpen ? closeDidForm() : openDidForm())}>
              {didFormOpen ? t("admin.trunks.btn.cancel") : t("admin.trunks.btn.adddid")}
            </button>
          )}
        </div>
      </div>

      <div className="filter-row">
        <button className={`chip-button${tab === "trunks" ? " active" : ""}`} onClick={() => setTab("trunks")}>
          {t("admin.trunks.tab.trunks", { n: trunksData.length })}
        </button>
        <button className={`chip-button${tab === "dids" ? " active" : ""}`} onClick={() => setTab("dids")}>
          {t("admin.trunks.tab.dids", { n: didsData.length })}
        </button>
      </div>

      {tab === "trunks" ? (
        <>
          {trunkFormOpen ? <TrunkForm initial={editingTrunk ?? undefined} onSubmit={handleSaveTrunk} onCancel={closeTrunkForm} /> : null}

          <div className="panel" style={{ marginBottom: 16 }}>
            <p className="subtle" style={{ marginBottom: 0 }}>{t("admin.trunks.realtime.hint", { num: "{numero}" })}</p>
          </div>

          <div className="table-wrap">
            {trunks.loading ? (
              <div className="empty-state">{t("g.loading")}</div>
            ) : trunksData.length === 0 ? (
              <div className="empty-state">{t("admin.trunks.empty.full")}</div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th>{t("admin.trunks.col.name")}</th>
                    <th>{t("admin.trunks.col.provider")}</th>
                    <th>{t("admin.trunks.col.endpoint")}</th>
                    <th>{t("admin.trunks.col.host")}</th>
                    <th>{t("admin.trunks.col.appstate")}</th>
                    <th>
                      {t("admin.trunks.col.sipstate")}{" "}
                      {ariEnabled === false ? <span className="subtle">{t("admin.trunks.col.ariofflabel")}</span> : null}
                    </th>
                    <th>{t("admin.trunks.col.dids")}</th>
                    <th>{t("admin.trunks.col.action")}</th>
                  </tr>
                </thead>
                <tbody>
                  {trunksData.map((trunk) => {
                    const sip = sipState[trunk.asteriskEndpoint];
                    return (
                      <tr key={trunk.id}>
                        <td className="primary-cell">{trunk.name}</td>
                        <td>
                          <span className="chip">{trunk.provider || "—"}</span>
                        </td>
                        <td>
                          <code className="mono">{trunk.asteriskEndpoint}</code>
                        </td>
                        <td>
                          {trunk.host || "—"}
                          {trunk.port ? `:${trunk.port}` : ""}
                        </td>
                        <td>
                          <span className={statusClass(trunk.status)}>{statusLabel(trunk.status)}</span>
                        </td>
                        <td>
                          <SipStateBadge state={sip?.state} ariEnabled={ariEnabled} />
                          {sip && sip.channels > 0 ? (
                            <span className="subtle" style={{ marginLeft: 8 }}>
                              {sip.channels === 1
                                ? t("admin.trunks.callcount.one", { n: sip.channels })
                                : t("admin.trunks.callcount.many", { n: sip.channels })}
                            </span>
                          ) : null}
                        </td>
                        <td>{trunk.didCount}</td>
                        <td>
                          <button className="button ghost compact" onClick={() => openTrunkForm(trunk)}>
                            {t("admin.trunks.btn.edit")}
                          </button>
                          <button
                            className="button ghost compact"
                            style={{ marginLeft: 6 }}
                            onClick={() => handleDeleteTrunk(trunk.id)}
                          >
                            {t("admin.trunks.btn.delete")}
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </>
      ) : (
        <>
          {didFormOpen ? (
            <DIDForm
              initial={editingDid ?? undefined}
              trunks={trunksData}
              tenants={tenantsData}
              onSubmit={handleSaveDID}
              onCancel={closeDidForm}
            />
          ) : null}

          <div className="table-wrap">
            {dids.loading ? (
              <div className="empty-state">{t("g.loading")}</div>
            ) : didsData.length === 0 ? (
              <div className="empty-state">{t("admin.trunks.dids.empty")}</div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th>{t("admin.trunks.dids.col.number")}</th>
                    <th>{t("admin.trunks.dids.col.label")}</th>
                    <th>{t("admin.trunks.dids.col.trunk")}</th>
                    <th>{t("admin.trunks.dids.col.tenant")}</th>
                    <th>{t("admin.trunks.dids.col.status")}</th>
                    <th>{t("admin.trunks.dids.col.actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {didsData.map((did) => (
                    <DIDRow
                      key={did.id}
                      did={did}
                      tenants={tenantsData}
                      onAssign={(tenantId) => handleAssignDID(did.id, tenantId)}
                      onEdit={() => openDidForm(did)}
                      onDelete={() => handleDeleteDID(did.id)}
                    />
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      )}
    </>
  );
}

function TrunkForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial?: SIPTrunk;
  onSubmit: (input: Partial<SIPTrunk>) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const editing = Boolean(initial);
  const [name, setName] = useState(initial?.name ?? "");
  const [provider, setProvider] = useState(initial?.provider || "twilio");
  const [asteriskEndpoint, setAsteriskEndpoint] = useState(initial?.asteriskEndpoint ?? "");
  const [host, setHost] = useState(initial?.host ?? "");
  const [port, setPort] = useState(initial?.port ?? 5060);
  const [username, setUsername] = useState(initial?.username ?? "");
  const [password, setPassword] = useState("");
  const [register, setRegister] = useState(initial ? initial.register : true);
  const [identifyIp, setIdentifyIp] = useState(initial?.identifyIp ?? "");
  const [notes, setNotes] = useState(initial?.notes ?? "");
  const [submitting, setSubmitting] = useState(false);

  return (
    <form
      className="panel"
      style={{ marginBottom: 16 }}
      onSubmit={async (e) => {
        e.preventDefault();
        setSubmitting(true);
        await onSubmit({
          name,
          provider,
          asteriskEndpoint,
          host,
          port,
          username,
          password,
          register,
          identifyIp,
          notes,
          status: "active",
        });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">{editing ? t("admin.trunks.form.edit.eyebrow") : t("admin.trunks.form.new.eyebrow")}</p>
          <h2>{editing ? `${initial?.name}` : t("admin.trunks.form.new.title")}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("admin.trunks.form.name")}</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder={t("admin.trunks.form.name.placeholder")} />
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.provider")}</label>
          <select value={provider} onChange={(e) => setProvider(e.target.value)}>
            <option value="twilio">Twilio</option>
            <option value="vonage">Vonage</option>
            <option value="telnyx">Telnyx</option>
            <option value="internal">{t("admin.trunks.form.provider.internal")}</option>
            <option value="custom">{t("admin.trunks.form.provider.custom")}</option>
          </select>
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.endpoint")}</label>
          <input
            value={asteriskEndpoint}
            onChange={(e) => setAsteriskEndpoint(e.target.value)}
            required
            placeholder="twilio-eu"
          />
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.host")}</label>
          <input value={host} onChange={(e) => setHost(e.target.value)} placeholder="sip.twilio.com" />
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.port")}</label>
          <input type="number" value={port} onChange={(e) => setPort(parseInt(e.target.value, 10) || 5060)} />
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.user")}</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder={t("admin.trunks.form.user.placeholder")} />
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.password")}</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={editing ? t("admin.trunks.form.password.keep") : "••••••••"}
          />
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.authmode")}</label>
          <select value={register ? "register" : "identify"} onChange={(e) => setRegister(e.target.value === "register")}>
            <option value="register">{t("admin.trunks.form.authmode.register")}</option>
            <option value="identify">{t("admin.trunks.form.authmode.identify")}</option>
          </select>
        </div>
        <div className="field">
          <label>{t("admin.trunks.form.identifyip")}</label>
          <input
            value={identifyIp}
            onChange={(e) => setIdentifyIp(e.target.value)}
            placeholder="54.172.60.0"
            disabled={register}
          />
        </div>
        <div className="field" style={{ gridColumn: "1 / -1" }}>
          <label>{t("admin.trunks.form.notes")}</label>
          <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          {t("admin.trunks.btn.cancel")}
        </button>
        <button className="button" disabled={submitting}>
          {submitting ? t("admin.trunks.form.submitting") : editing ? t("admin.trunks.form.submit.edit") : t("admin.trunks.form.submit.create")}
        </button>
      </div>
    </form>
  );
}

function DIDForm({
  initial,
  trunks,
  tenants,
  onSubmit,
  onCancel,
}: {
  initial?: DID;
  trunks: SIPTrunk[];
  tenants: Tenant[];
  onSubmit: (input: Parameters<typeof api.adminCreateDID>[0]) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const editing = Boolean(initial);
  const [trunkId, setTrunkId] = useState(initial?.trunkId ?? trunks[0]?.id ?? "");
  const [e164, setE164] = useState(initial?.e164 ?? "");
  const [label, setLabel] = useState(initial?.label ?? "");
  const [tenantId, setTenantId] = useState(initial?.tenantId ?? "");
  const [status, setStatus] = useState(initial?.status ?? "active");
  const [submitting, setSubmitting] = useState(false);

  if (trunks.length === 0) {
    return (
      <div className="empty-state" style={{ marginBottom: 16 }}>
        {t("admin.trunks.dids.form.notrunks")}
      </div>
    );
  }

  return (
    <form
      className="panel"
      style={{ marginBottom: 16 }}
      onSubmit={async (e) => {
        e.preventDefault();
        setSubmitting(true);
        await onSubmit({
          trunkId,
          e164,
          label,
          status,
          tenantId: tenantId || null,
        });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">{editing ? t("admin.trunks.dids.form.edit.eyebrow") : t("admin.trunks.dids.form.new.eyebrow")}</p>
          <h2>{editing ? initial?.e164 : t("admin.trunks.dids.form.new.title")}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("admin.trunks.dids.form.e164")}</label>
          <input value={e164} onChange={(e) => setE164(e.target.value)} required placeholder="+34911000000" />
        </div>
        <div className="field">
          <label>{t("admin.trunks.dids.form.label")}</label>
          <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={t("admin.trunks.dids.form.label.placeholder")} />
        </div>
        <div className="field">
          <label>{t("admin.trunks.dids.form.trunk")}</label>
          <select value={trunkId} onChange={(e) => setTrunkId(e.target.value)} required disabled={editing}>
            {trunks.map((tr) => (
              <option key={tr.id} value={tr.id}>
                {tr.name} ({tr.asteriskEndpoint})
              </option>
            ))}
          </select>
          {editing ? (
            <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>{t("admin.trunks.dids.form.trunk.locked")}</p>
          ) : null}
        </div>
        <div className="field">
          <label>{t("admin.trunks.dids.form.tenant")}</label>
          <select value={tenantId ?? ""} onChange={(e) => setTenantId(e.target.value)} disabled={editing}>
            <option value="">{t("admin.trunks.dids.form.tenant.unassigned")}</option>
            {tenants.map((tn) => (
              <option key={tn.id} value={tn.id}>
                {tn.name}
              </option>
            ))}
          </select>
          {editing ? (
            <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>{t("admin.trunks.dids.form.tenant.locked")}</p>
          ) : null}
        </div>
        <div className="field">
          <label>{t("admin.trunks.dids.form.status")}</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="active">{t("admin.trunks.dids.form.status.active")}</option>
            <option value="disabled">{t("admin.trunks.dids.form.status.disabled")}</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          {t("admin.trunks.btn.cancel")}
        </button>
        <button className="button" disabled={submitting}>
          {submitting ? t("admin.trunks.form.submitting") : editing ? t("admin.trunks.dids.form.submit.edit") : t("admin.trunks.dids.form.submit.create")}
        </button>
      </div>
    </form>
  );
}

function SipStateBadge({ state, ariEnabled }: { state?: string; ariEnabled: boolean | null }) {
  const t = useT();
  if (ariEnabled === false) {
    return <span className="status">—</span>;
  }
  if (!state) {
    return <span className="status warn">{t("admin.trunks.sipstate.unknown")}</span>;
  }
  if (state === "online") return <span className="status good">{t("admin.trunks.sipstate.registered")}</span>;
  if (state === "offline") return <span className="status danger">{t("admin.trunks.sipstate.down")}</span>;
  return <span className="status">{state}</span>;
}

function DIDRow({
  did,
  tenants,
  onAssign,
  onEdit,
  onDelete,
}: {
  did: DID;
  tenants: Tenant[];
  onAssign: (tenantId: string | null) => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  const statusLabel = useStatusLabel();
  return (
    <tr>
      <td className="primary-cell">
        <code className="mono">{did.e164}</code>
      </td>
      <td>{did.label || "—"}</td>
      <td>
        <span className="chip">{did.trunkName || did.trunkId}</span>
      </td>
      <td>
        <select
          value={did.tenantId ?? ""}
          onChange={(e) => onAssign(e.target.value || null)}
          className="inline-select"
        >
          <option value="">{t("admin.trunks.dids.pool")}</option>
          {tenants.map((tn) => (
            <option key={tn.id} value={tn.id}>
              {tn.name}
            </option>
          ))}
        </select>
      </td>
      <td>
        <span className={statusClass(did.status)}>{statusLabel(did.status)}</span>
      </td>
      <td>
        <button className="button ghost compact" onClick={onEdit}>
          {t("admin.trunks.btn.edit")}
        </button>
        <button className="button ghost compact" style={{ marginLeft: 6 }} onClick={onDelete}>
          {t("admin.trunks.btn.delete")}
        </button>
      </td>
    </tr>
  );
}
