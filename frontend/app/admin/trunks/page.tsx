"use client";

import { useEffect, useState } from "react";
import { useToast } from "../../../components/toast";
import { api, ApiError, DID, SIPTrunk, Tenant, statusClass } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

type Tab = "trunks" | "dids";

type EndpointState = { state: string; channels: number };

export default function TrunksPage() {
  const { user } = useAuth();
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

  // Poll real-time SIP registration state from Asterisk every 10s while the
  // trunks tab is visible. Cheap call (HTTP to ARI on the docker network).
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
    return <div className="empty-state danger">Acceso restringido al rol platform_admin.</div>;
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
        toast.push("Trunk actualizado", "success");
      } else {
        await api.adminCreateTrunk(input);
        toast.push("Trunk creado", "success");
      }
      closeTrunkForm();
      reloadAll();
    } catch (err) {
      toast.push(`No se pudo guardar: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleDeleteTrunk(id: string) {
    if (!confirm("Eliminar trunk? Solo es posible si no tiene DIDs asociados.")) return;
    try {
      await api.adminDeleteTrunk(id);
      toast.push("Trunk eliminado", "success");
      reloadAll();
    } catch (err) {
      toast.push(`No se pudo eliminar: ${err instanceof ApiError ? err.code : "error"}`, "danger");
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
        toast.push("DID actualizado", "success");
      } else {
        await api.adminCreateDID(input);
        toast.push("DID añadido", "success");
      }
      closeDidForm();
      reloadAll();
    } catch (err) {
      toast.push(`No se pudo guardar: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleAssignDID(id: string, tenantId: string | null) {
    try {
      await api.adminAssignDID(id, tenantId);
      toast.push(tenantId ? "DID asignado" : "DID liberado al pool", "success");
      dids.reload();
    } catch (err) {
      toast.push(`No se pudo asignar: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleDeleteDID(id: string) {
    if (!confirm("Eliminar este DID? Se desasigna del bot si lo tiene.")) return;
    try {
      await api.adminDeleteDID(id);
      toast.push("DID eliminado", "success");
      reloadAll();
    } catch (err) {
      toast.push(`No se pudo eliminar: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  const trunksData = trunks.data ?? [];
  const didsData = dids.data ?? [];
  const tenantsData = tenants.data ?? [];

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Trunks SIP y DIDs</h1>
          <p className="subtle">
            Configura los trunks de tu proveedor SIP y los numeros (DIDs) que asignas a cada cliente. Cada tenant decide
            que DID usa cada bot.
          </p>
        </div>
        <div className="actions">
          {tab === "trunks" ? (
            <button className="button" onClick={() => (trunkFormOpen ? closeTrunkForm() : openTrunkForm())}>
              {trunkFormOpen ? "Cancelar" : "Nuevo trunk"}
            </button>
          ) : (
            <button className="button" onClick={() => (didFormOpen ? closeDidForm() : openDidForm())}>
              {didFormOpen ? "Cancelar" : "Añadir DID"}
            </button>
          )}
        </div>
      </div>

      <div className="filter-row">
        <button className={`chip-button${tab === "trunks" ? " active" : ""}`} onClick={() => setTab("trunks")}>
          Trunks ({trunksData.length})
        </button>
        <button className={`chip-button${tab === "dids" ? " active" : ""}`} onClick={() => setTab("dids")}>
          DIDs ({didsData.length})
        </button>
      </div>

      {tab === "trunks" ? (
        <>
          {trunkFormOpen ? <TrunkForm initial={editingTrunk ?? undefined} onSubmit={handleSaveTrunk} onCancel={closeTrunkForm} /> : null}

          <div className="panel" style={{ marginBottom: 16 }}>
            <p className="subtle" style={{ marginBottom: 0 }}>
              Asterisk lee los trunks <strong>en vivo desde Postgres</strong> (PJSIP Realtime). Al crear o editar
              un trunk aquí, los cambios se aplican sin reiniciar Asterisk. El campo <code>endpoint</code> es el
              identificador interno que usarás en la marcación (p.ej. <code>PJSIP/{`{numero}`}@twilio-eu</code>).
            </p>
          </div>

          <div className="table-wrap">
            {trunks.loading ? (
              <div className="empty-state">Cargando…</div>
            ) : trunksData.length === 0 ? (
              <div className="empty-state">Aun no hay trunks. Crea el primero.</div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th>Nombre</th>
                    <th>Proveedor</th>
                    <th>Endpoint</th>
                    <th>Host</th>
                    <th>Estado app</th>
                    <th>Estado SIP {ariEnabled === false ? <span className="subtle">(ARI off)</span> : null}</th>
                    <th>DIDs</th>
                    <th>Accion</th>
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
                          <span className={statusClass(trunk.status)}>{trunk.status}</span>
                        </td>
                        <td>
                          <SipStateBadge state={sip?.state} ariEnabled={ariEnabled} />
                          {sip && sip.channels > 0 ? (
                            <span className="subtle" style={{ marginLeft: 8 }}>
                              {sip.channels} llamada{sip.channels === 1 ? "" : "s"}
                            </span>
                          ) : null}
                        </td>
                        <td>{trunk.didCount}</td>
                        <td>
                          <button className="button ghost compact" onClick={() => openTrunkForm(trunk)}>
                            Editar
                          </button>
                          <button
                            className="button ghost compact"
                            style={{ marginLeft: 6 }}
                            onClick={() => handleDeleteTrunk(trunk.id)}
                          >
                            Eliminar
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
              <div className="empty-state">Cargando…</div>
            ) : didsData.length === 0 ? (
              <div className="empty-state">Aun no hay DIDs. Crea trunks y añade numeros.</div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th>Numero</th>
                    <th>Etiqueta</th>
                    <th>Trunk</th>
                    <th>Tenant</th>
                    <th>Estado</th>
                    <th>Acciones</th>
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
  const editing = Boolean(initial);
  const [name, setName] = useState(initial?.name ?? "");
  const [provider, setProvider] = useState(initial?.provider || "twilio");
  const [asteriskEndpoint, setAsteriskEndpoint] = useState(initial?.asteriskEndpoint ?? "");
  const [host, setHost] = useState(initial?.host ?? "");
  const [port, setPort] = useState(initial?.port ?? 5060);
  const [username, setUsername] = useState(initial?.username ?? "");
  // Al editar, el backend nos devuelve "********" como password. Dejamos el
  // campo vacío y un placeholder claro: si no se rellena, conservamos el actual.
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
          <p className="eyebrow">{editing ? "Editar trunk" : "Nuevo trunk"}</p>
          <h2>{editing ? `${initial?.name}` : "Conectar proveedor SIP"}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Nombre interno</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Twilio Europe" />
        </div>
        <div className="field">
          <label>Proveedor</label>
          <select value={provider} onChange={(e) => setProvider(e.target.value)}>
            <option value="twilio">Twilio</option>
            <option value="vonage">Vonage</option>
            <option value="telnyx">Telnyx</option>
            <option value="internal">Interno / sandbox</option>
            <option value="custom">Custom</option>
          </select>
        </div>
        <div className="field">
          <label>Endpoint (identificador interno)</label>
          <input
            value={asteriskEndpoint}
            onChange={(e) => setAsteriskEndpoint(e.target.value)}
            required
            placeholder="twilio-eu"
          />
        </div>
        <div className="field">
          <label>Host del proveedor</label>
          <input value={host} onChange={(e) => setHost(e.target.value)} placeholder="sip.twilio.com" />
        </div>
        <div className="field">
          <label>Puerto</label>
          <input type="number" value={port} onChange={(e) => setPort(parseInt(e.target.value, 10) || 5060)} />
        </div>
        <div className="field">
          <label>Usuario SIP</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="ACxxxxxxx (Twilio SID)" />
        </div>
        <div className="field">
          <label>Contraseña SIP</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={editing ? "Dejar vacío = conservar la actual" : "••••••••"}
          />
        </div>
        <div className="field">
          <label>Modo de autenticación</label>
          <select value={register ? "register" : "identify"} onChange={(e) => setRegister(e.target.value === "register")}>
            <option value="register">REGISTER (Twilio Programmable Voice, Vonage)</option>
            <option value="identify">IP Identify (Twilio Elastic SIP Trunking, Telnyx)</option>
          </select>
        </div>
        <div className="field">
          <label>IP del proveedor (solo modo Identify)</label>
          <input
            value={identifyIp}
            onChange={(e) => setIdentifyIp(e.target.value)}
            placeholder="54.172.60.0"
            disabled={register}
          />
        </div>
        <div className="field" style={{ gridColumn: "1 / -1" }}>
          <label>Notas</label>
          <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          Cancelar
        </button>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : editing ? "Guardar cambios" : "Crear trunk"}
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
        Crea primero un trunk antes de añadir DIDs.
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
          <p className="eyebrow">{editing ? "Editar DID" : "Nuevo DID"}</p>
          <h2>{editing ? initial?.e164 : "Añadir numero"}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Numero E.164</label>
          <input value={e164} onChange={(e) => setE164(e.target.value)} required placeholder="+34911000000" />
        </div>
        <div className="field">
          <label>Etiqueta</label>
          <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Madrid - alquiler" />
        </div>
        <div className="field">
          <label>Trunk</label>
          <select value={trunkId} onChange={(e) => setTrunkId(e.target.value)} required disabled={editing}>
            {trunks.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} ({t.asteriskEndpoint})
              </option>
            ))}
          </select>
          {editing ? (
            <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>
              El trunk no se puede cambiar tras crear el DID.
            </p>
          ) : null}
        </div>
        <div className="field">
          <label>Asignar a tenant (opcional)</label>
          <select value={tenantId ?? ""} onChange={(e) => setTenantId(e.target.value)} disabled={editing}>
            <option value="">— Sin asignar (pool) —</option>
            {tenants.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
          {editing ? (
            <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>
              Asigna desde el selector de la fila.
            </p>
          ) : null}
        </div>
        <div className="field">
          <label>Estado</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="active">Activo</option>
            <option value="disabled">Deshabilitado</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12, gap: 8 }}>
        <button type="button" className="button ghost" onClick={onCancel} disabled={submitting}>
          Cancelar
        </button>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : editing ? "Guardar cambios" : "Añadir DID"}
        </button>
      </div>
    </form>
  );
}

function SipStateBadge({ state, ariEnabled }: { state?: string; ariEnabled: boolean | null }) {
  if (ariEnabled === false) {
    return <span className="status">—</span>;
  }
  if (!state) {
    // ARI activo pero Asterisk no lo ve aún (caché, o trunk recién creado).
    return <span className="status warn">desconocido</span>;
  }
  if (state === "online") return <span className="status good">registrado</span>;
  if (state === "offline") return <span className="status danger">caído</span>;
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
          <option value="">— Pool —</option>
          {tenants.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </td>
      <td>
        <span className={statusClass(did.status)}>{did.status}</span>
      </td>
      <td>
        <button className="button ghost compact" onClick={onEdit}>
          Editar
        </button>
        <button className="button ghost compact" style={{ marginLeft: 6 }} onClick={onDelete}>
          Eliminar
        </button>
      </td>
    </tr>
  );
}
