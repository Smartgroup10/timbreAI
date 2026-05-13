"use client";

import { useState } from "react";
import { useToast } from "../../../components/toast";
import { api, ApiError, DID, SIPTrunk, Tenant, statusClass } from "../../../lib/api";
import { useAuth } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

type Tab = "trunks" | "dids";

export default function TrunksPage() {
  const { user } = useAuth();
  const trunks = useResource(() => api.adminTrunks(), []);
  const dids = useResource(() => api.adminDIDs(), []);
  const tenants = useResource(() => api.tenants(), []);
  const [tab, setTab] = useState<Tab>("trunks");
  const [trunkFormOpen, setTrunkFormOpen] = useState(false);
  const [didFormOpen, setDidFormOpen] = useState(false);
  const toast = useToast();

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">Acceso restringido al rol platform_admin.</div>;
  }

  function reloadAll() {
    trunks.reload();
    dids.reload();
  }

  async function handleCreateTrunk(input: Partial<SIPTrunk>) {
    try {
      await api.adminCreateTrunk(input);
      toast.push("Trunk creado", "success");
      setTrunkFormOpen(false);
      reloadAll();
    } catch (err) {
      toast.push(`No se pudo crear: ${err instanceof ApiError ? err.code : "error"}`, "danger");
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

  async function handleCreateDID(input: Parameters<typeof api.adminCreateDID>[0]) {
    try {
      await api.adminCreateDID(input);
      toast.push("DID añadido", "success");
      setDidFormOpen(false);
      reloadAll();
    } catch (err) {
      toast.push(`No se pudo crear: ${err instanceof ApiError ? err.code : "error"}`, "danger");
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
            <button className="button" onClick={() => setTrunkFormOpen((v) => !v)}>
              {trunkFormOpen ? "Cancelar" : "Nuevo trunk"}
            </button>
          ) : (
            <button className="button" onClick={() => setDidFormOpen((v) => !v)}>
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
          {trunkFormOpen ? <TrunkForm onSubmit={handleCreateTrunk} /> : null}

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
                    <th>Endpoint Asterisk</th>
                    <th>Host</th>
                    <th>Estado</th>
                    <th>DIDs</th>
                    <th>Accion</th>
                  </tr>
                </thead>
                <tbody>
                  {trunksData.map((trunk) => (
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
                      <td>{trunk.didCount}</td>
                      <td>
                        <button className="button ghost compact" onClick={() => handleDeleteTrunk(trunk.id)}>
                          Eliminar
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      ) : (
        <>
          {didFormOpen ? <DIDForm trunks={trunksData} tenants={tenantsData} onSubmit={handleCreateDID} /> : null}

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

function TrunkForm({ onSubmit }: { onSubmit: (input: Partial<SIPTrunk>) => Promise<void> }) {
  const [name, setName] = useState("");
  const [provider, setProvider] = useState("twilio");
  const [asteriskEndpoint, setAsteriskEndpoint] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState(5060);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [register, setRegister] = useState(true);
  const [identifyIp, setIdentifyIp] = useState("");
  const [notes, setNotes] = useState("");
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
          <p className="eyebrow">Nuevo trunk</p>
          <h2>Conectar proveedor SIP</h2>
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
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="••••••••" />
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
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : "Crear trunk"}
        </button>
      </div>
    </form>
  );
}

function DIDForm({
  trunks,
  tenants,
  onSubmit,
}: {
  trunks: SIPTrunk[];
  tenants: Tenant[];
  onSubmit: (input: Parameters<typeof api.adminCreateDID>[0]) => Promise<void>;
}) {
  const [trunkId, setTrunkId] = useState(trunks[0]?.id ?? "");
  const [e164, setE164] = useState("");
  const [label, setLabel] = useState("");
  const [tenantId, setTenantId] = useState("");
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
          tenantId: tenantId || null,
        });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Nuevo DID</p>
          <h2>Añadir numero</h2>
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
          <select value={trunkId} onChange={(e) => setTrunkId(e.target.value)} required>
            {trunks.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} ({t.asteriskEndpoint})
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>Asignar a tenant (opcional)</label>
          <select value={tenantId} onChange={(e) => setTenantId(e.target.value)}>
            <option value="">— Sin asignar (pool) —</option>
            {tenants.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : "Añadir DID"}
        </button>
      </div>
    </form>
  );
}

function DIDRow({
  did,
  tenants,
  onAssign,
  onDelete,
}: {
  did: DID;
  tenants: Tenant[];
  onAssign: (tenantId: string | null) => void;
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
        <button className="button ghost compact" onClick={onDelete}>
          Eliminar
        </button>
      </td>
    </tr>
  );
}
