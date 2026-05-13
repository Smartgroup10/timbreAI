"use client";

import { useState } from "react";
import { useToast } from "../../../components/toast";
import { api, ApiError } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

export default function DoNotCallPage() {
  const tenant = useTenantScope();
  const dnc = useResource(() => api.dnc(tenant), [tenant]);
  const toast = useToast();
  const [phone, setPhone] = useState("");
  const [reason, setReason] = useState("opt_out");
  const [submitting, setSubmitting] = useState(false);

  async function handleAdd(event: React.FormEvent) {
    event.preventDefault();
    if (!phone.trim()) return;
    setSubmitting(true);
    try {
      await api.addDNC({ phone: phone.trim(), reason }, tenant);
      toast.push("Número añadido a la lista", "success");
      setPhone("");
      dnc.reload();
    } catch (err) {
      toast.push(`No se pudo añadir: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRemove(id: string, phoneNumber: string) {
    if (!confirm(`Eliminar ${phoneNumber} de la lista? Volverá a poder recibir llamadas.`)) return;
    try {
      await api.removeDNC(id, tenant);
      toast.push("Número liberado", "success");
      dnc.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  const entries = dnc.data ?? [];

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Do Not Call</h1>
          <p className="subtle">
            Números bloqueados para llamadas salientes. Cada intento de originar una llamada hacia uno de
            estos números es rechazado por el backend antes de tocar Asterisk.
          </p>
        </div>
      </div>

      <form className="panel" style={{ marginBottom: 16 }} onSubmit={handleAdd}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">Añadir número</p>
            <h2>Bloquear un teléfono</h2>
          </div>
        </div>
        <div className="form-grid">
          <div className="field">
            <label>Teléfono (E.164)</label>
            <input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+34600123456" required />
          </div>
          <div className="field">
            <label>Motivo</label>
            <select value={reason} onChange={(e) => setReason(e.target.value)}>
              <option value="opt_out">El destinatario pidió no recibir llamadas</option>
              <option value="complaint">Queja</option>
              <option value="legal">Requerimiento legal</option>
              <option value="manual">Bloqueo manual</option>
            </select>
          </div>
        </div>
        <div className="actions" style={{ marginTop: 12 }}>
          <button className="button" disabled={submitting}>
            {submitting ? "Guardando…" : "Bloquear número"}
          </button>
        </div>
      </form>

      <div className="table-wrap">
        {dnc.loading ? (
          <div className="empty-state">Cargando…</div>
        ) : entries.length === 0 ? (
          <div className="empty-state">Aún no hay números bloqueados.</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Teléfono</th>
                <th>Motivo</th>
                <th>Añadido</th>
                <th>Acción</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr key={e.id}>
                  <td className="primary-cell">
                    <code className="mono">{e.phone}</code>
                  </td>
                  <td>
                    <span className="chip">{e.reason}</span>
                  </td>
                  <td>{new Date(e.createdAt).toLocaleString()}</td>
                  <td>
                    <button className="button ghost compact" onClick={() => handleRemove(e.id, e.phone)}>
                      Liberar
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
