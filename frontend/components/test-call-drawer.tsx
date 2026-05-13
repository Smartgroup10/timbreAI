"use client";

import { useEffect, useState } from "react";
import { api, ApiError, Bot, TestCallResponse } from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useToast } from "./toast";

type Props = {
  open: boolean;
  onClose: () => void;
  defaultPhone?: string;
  defaultLeadName?: string;
  defaultBotId?: string;
  onCallCreated?: (response: TestCallResponse) => void;
};

export function TestCallDrawer({
  open,
  onClose,
  defaultPhone = "",
  defaultLeadName = "",
  defaultBotId = "",
  onCallCreated,
}: Props) {
  const tenant = useTenantScope();
  const [phone, setPhone] = useState(defaultPhone);
  const [leadName, setLeadName] = useState(defaultLeadName);
  const [botId, setBotId] = useState(defaultBotId);
  const [bots, setBots] = useState<Bot[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [response, setResponse] = useState<TestCallResponse | null>(null);
  const toast = useToast();

  useEffect(() => {
    if (!open) return;
    setPhone(defaultPhone);
    setLeadName(defaultLeadName);
    setBotId(defaultBotId);
    setResponse(null);
    api.bots(tenant).then(setBots).catch(() => setBots([]));
  }, [open, defaultPhone, defaultLeadName, defaultBotId, tenant]);

  if (!open) return null;

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!phone.trim()) {
      toast.push("Numero de telefono requerido", "warn");
      return;
    }
    setSubmitting(true);
    try {
      const res = await api.testCall({
        phone: phone.trim(),
        leadName: leadName.trim() || undefined,
        botId: botId || undefined,
      });
      setResponse(res);
      onCallCreated?.(res);
      toast.push(res.channel ? `Canal ${res.channel.id} originado` : "Llamada registrada", "success");
    } catch (err) {
      const message = err instanceof ApiError ? err.code : "Error desconocido";
      toast.push(`No se pudo originar la llamada: ${message}`, "danger");
    } finally {
      setSubmitting(false);
    }
  }

  const botsWithDID = bots.filter((b) => b.didId);
  const botsWithoutDID = bots.filter((b) => !b.didId);
  const selectedBot = bots.find((b) => b.id === botId);

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label="Cerrar" />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">Llamada de prueba</p>
            <h2>Originar canal via ARI</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            Cerrar
          </button>
        </header>

        <form className="drawer-body" onSubmit={handleSubmit}>
          <div className="field">
            <label htmlFor="phone">Telefono destino</label>
            <input
              id="phone"
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="+34 600 000 000"
              required
            />
          </div>
          <div className="field">
            <label htmlFor="leadName">Nombre o etiqueta (opcional)</label>
            <input
              id="leadName"
              type="text"
              value={leadName}
              onChange={(e) => setLeadName(e.target.value)}
              placeholder="Nombre del contacto"
            />
          </div>
          <div className="field">
            <label htmlFor="bot">Bot saliente</label>
            <select id="bot" value={botId} onChange={(e) => setBotId(e.target.value)}>
              <option value="">— Sandbox interno (PJSIP/6001) —</option>
              {botsWithDID.length > 0 ? (
                <optgroup label="Bots con DID asignado">
                  {botsWithDID.map((b) => (
                    <option key={b.id} value={b.id}>
                      {b.name} ({b.didE164})
                    </option>
                  ))}
                </optgroup>
              ) : null}
              {botsWithoutDID.length > 0 ? (
                <optgroup label="Bots sin DID (no podran llamar)">
                  {botsWithoutDID.map((b) => (
                    <option key={b.id} value={b.id} disabled>
                      {b.name} — sin DID
                    </option>
                  ))}
                </optgroup>
              ) : null}
            </select>
            {selectedBot?.didE164 ? (
              <p className="subtle" style={{ marginTop: 4 }}>
                Saldra como <code className="mono">{selectedBot.didE164}</code> via trunk asignado.
              </p>
            ) : (
              <p className="subtle" style={{ marginTop: 4 }}>
                Sin bot: se usa la extension de sandbox interna del backend.
              </p>
            )}
          </div>

          <button className="button" disabled={submitting}>
            {submitting ? "Originando…" : "Lanzar llamada"}
          </button>
        </form>

        {response ? (
          <div className="drawer-result">
            <p className="eyebrow">Resultado</p>
            <div className="kv">
              <span>Call ID</span>
              <strong>{response.call.id}</strong>
            </div>
            <div className="kv">
              <span>Status</span>
              <strong>{response.call.status}</strong>
            </div>
            {response.channel ? (
              <div className="kv">
                <span>Channel</span>
                <strong>{response.channel.id}</strong>
              </div>
            ) : null}
            {response.endpoint ? (
              <div className="kv">
                <span>Endpoint</span>
                <strong>{response.endpoint}</strong>
              </div>
            ) : null}
            {response.message ? <p className="subtle">{response.message}</p> : null}
          </div>
        ) : null}
      </aside>
    </div>
  );
}
