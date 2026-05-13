"use client";

import { useEffect, useState } from "react";
import { Pause, Play, Trash2, Users } from "lucide-react";
import { useToast } from "../../../components/toast";
import { api, ApiError, Bot, Campaign, CampaignLead, Lead, statusClass } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

// Convierte un timestamp ISO del backend a lo que espera <input type="datetime-local">
// (formato "YYYY-MM-DDTHH:mm" en zona local). Devuelve "" si la fecha es null.
function toLocalInput(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// Convierte lo que el navegador devuelve en datetime-local (zona local) a ISO 8601
// con offset. Empty = null (para "clear").
function fromLocalInput(v: string): string | null {
  if (!v) return null;
  const d = new Date(v);
  if (isNaN(d.getTime())) return null;
  return d.toISOString();
}

export default function CampaignsPage() {
  const tenant = useTenantScope();
  const campaigns = useResource(() => api.campaigns(tenant), [tenant]);
  const bots = useResource(() => api.bots(tenant), [tenant]);
  const [formOpen, setFormOpen] = useState(false);
  const [leadsDrawer, setLeadsDrawer] = useState<Campaign | null>(null);
  const toast = useToast();

  async function handleCreate(input: Partial<Campaign>) {
    try {
      await api.createCampaign(input);
      toast.push("Campaña creada", "success");
      setFormOpen(false);
      campaigns.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(`No se pudo crear: ${code}`, "danger");
    }
  }

  async function handleLaunch(c: Campaign) {
    try {
      await api.updateCampaign(c.id, { status: "active" }, tenant);
      toast.push("Campaña lanzada — el worker empezará a marcar en <30s", "success");
      campaigns.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handlePause(c: Campaign) {
    try {
      await api.updateCampaign(c.id, { status: "paused" }, tenant);
      toast.push("Campaña pausada", "success");
      campaigns.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleDelete(c: Campaign) {
    if (!confirm(`Eliminar la campaña "${c.name}"?`)) return;
    try {
      await api.deleteCampaign(c.id, tenant);
      toast.push("Campaña eliminada", "success");
      campaigns.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Campañas</h1>
          <p className="subtle">
            Programa cuándo se lanza cada campaña, cuántas llamadas en paralelo y respeta los horarios
            permitidos del tenant.
          </p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setFormOpen((v) => !v)}>
            {formOpen ? "Cancelar" : "Nueva campaña"}
          </button>
        </div>
      </div>

      {formOpen ? <CampaignForm bots={bots.data ?? []} onSubmit={handleCreate} /> : null}

      <div className="grid two" style={{ marginBottom: 16 }}>
        {(campaigns.data ?? []).map((campaign) => (
          <section className="panel" key={campaign.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">Campaña</p>
                <h2>{campaign.name}</h2>
              </div>
              <span className={statusClass(campaign.status)}>{campaign.status}</span>
            </div>
            <div className="command-strip">
              <div className="command-row">
                <span>Inicio</span>
                <strong>{campaign.startAt ? new Date(campaign.startAt).toLocaleString() : "Inmediato"}</strong>
              </div>
              <div className="command-row">
                <span>Fin</span>
                <strong>{campaign.endAt ? new Date(campaign.endAt).toLocaleString() : "Sin límite"}</strong>
              </div>
              <div className="command-row">
                <span>Bot</span>
                <strong>{(bots.data ?? []).find((b) => b.id === campaign.botId)?.name || campaign.botId || "—"}</strong>
              </div>
              <div className="command-row">
                <span>Leads</span>
                <strong>{campaign.leadCount}</strong>
              </div>
              <div className="command-row">
                <span>Concurrencia</span>
                <strong>{campaign.maxConcurrent} en paralelo</strong>
              </div>
              <div className="command-row">
                <span>Reintentos</span>
                <strong>{campaign.maxAttempts}</strong>
              </div>
            </div>
            <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
              <button className="button compact" onClick={() => setLeadsDrawer(campaign)}>
                <Users aria-hidden="true" />
                <span>Gestionar leads</span>
              </button>
              {campaign.status === "active" ? (
                <button className="button secondary compact" onClick={() => handlePause(campaign)}>
                  <Pause aria-hidden="true" />
                  <span>Pausar</span>
                </button>
              ) : (
                <button className="button secondary compact" onClick={() => handleLaunch(campaign)}>
                  <Play aria-hidden="true" />
                  <span>Lanzar</span>
                </button>
              )}
              <button className="button ghost compact" onClick={() => handleDelete(campaign)}>
                <Trash2 aria-hidden="true" />
                <span>Eliminar</span>
              </button>
            </div>
          </section>
        ))}
      </div>

      {leadsDrawer ? (
        <CampaignLeadsDrawer
          campaign={leadsDrawer}
          tenant={tenant}
          onClose={() => setLeadsDrawer(null)}
          onChanged={() => campaigns.reload()}
        />
      ) : null}

      <section className="panel">
        <div className="panel-header">
          <div>
            <p className="eyebrow">Control de lanzamiento</p>
            <h2>El worker hace esto antes de cada llamada</h2>
          </div>
        </div>
        <div className="grid three">
          <div>
            <h3>Consentimiento</h3>
            <p className="subtle">Cruza cada lead con la tabla DNC del tenant. Si está bloqueado, lo salta.</p>
          </div>
          <div>
            <h3>Horario</h3>
            <p className="subtle">
              Solo marca dentro de allowed_hours y allowed_days configurados en /portal/settings, en la
              zona horaria del tenant.
            </p>
          </div>
          <div>
            <h3>Volumen</h3>
            <p className="subtle">
              Respeta el daily cap del tenant y el max_concurrent de cada campaña (semáforo por campaña).
            </p>
          </div>
        </div>
      </section>
    </>
  );
}

function CampaignForm({ bots, onSubmit }: { bots: Bot[]; onSubmit: (input: Partial<Campaign>) => Promise<void> }) {
  const [name, setName] = useState("");
  const [botId, setBotId] = useState(bots[0]?.id ?? "");
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const [maxConcurrent, setMaxConcurrent] = useState(3);
  const [maxAttempts, setMaxAttempts] = useState(2);
  const [status, setStatus] = useState("draft");
  const [submitting, setSubmitting] = useState(false);

  return (
    <form
      className="panel"
      style={{ marginBottom: 16 }}
      onSubmit={async (event) => {
        event.preventDefault();
        setSubmitting(true);
        await onSubmit({
          name,
          botId,
          status,
          maxAttempts,
          maxConcurrent,
          startAt: fromLocalInput(startAt),
          endAt: fromLocalInput(endAt),
          // Mantengo schedule como label cosmético (se sigue mostrando si no usas pickers).
          schedule: startAt ? `${startAt} → ${endAt || "∞"}` : "",
        });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Nueva campaña</p>
          <h2>Programar</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Nombre</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Outbound enero leads frios" />
        </div>
        <div className="field">
          <label>Bot</label>
          <select value={botId} onChange={(e) => setBotId(e.target.value)} required>
            <option value="">—</option>
            {bots.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>Empieza a marcar (opcional)</label>
          <input type="datetime-local" value={startAt} onChange={(e) => setStartAt(e.target.value)} />
          <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>
            Vacío = empieza al pulsar "Lanzar". El worker respeta también el horario del tenant.
          </p>
        </div>
        <div className="field">
          <label>Termina (opcional)</label>
          <input type="datetime-local" value={endAt} onChange={(e) => setEndAt(e.target.value)} />
          <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>
            Vacío = corre hasta agotar leads.
          </p>
        </div>
        <div className="field">
          <label>Llamadas en paralelo</label>
          <input
            type="number"
            min={1}
            max={50}
            value={maxConcurrent}
            onChange={(e) => setMaxConcurrent(parseInt(e.target.value, 10) || 1)}
          />
          <p className="subtle" style={{ marginTop: 4, fontSize: 12 }}>
            Limitado además por puertos RTP de Asterisk y plan del proveedor SIP.
          </p>
        </div>
        <div className="field">
          <label>Intentos máximos por lead</label>
          <input
            type="number"
            min={1}
            max={10}
            value={maxAttempts}
            onChange={(e) => setMaxAttempts(parseInt(e.target.value, 10) || 1)}
          />
        </div>
        <div className="field">
          <label>Estado inicial</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="draft">Draft (no marca)</option>
            <option value="active">Active (empieza ya)</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : "Crear campaña"}
        </button>
      </div>
    </form>
  );
}

function CampaignLeadsDrawer({
  campaign,
  tenant,
  onClose,
  onChanged,
}: {
  campaign: Campaign;
  tenant: string | undefined;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [leads, setLeads] = useState<CampaignLead[]>([]);
  const [available, setAvailable] = useState<Lead[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const toast = useToast();

  async function reload() {
    setLoading(true);
    try {
      const [campLeads, allLeads] = await Promise.all([
        api.campaignLeads(campaign.id, tenant),
        api.leads(tenant),
      ]);
      setLeads(campLeads);
      const inCampaign = new Set(campLeads.map((cl) => cl.leadId));
      setAvailable(allLeads.filter((l) => !inCampaign.has(l.id)));
    } catch (err) {
      toast.push(`Error cargando leads: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [campaign.id]);

  async function handleAdd() {
    if (selected.size === 0) return;
    setAdding(true);
    try {
      const r = await api.addCampaignLeads(campaign.id, Array.from(selected), tenant);
      toast.push(`${r.created} leads añadidos`, "success");
      setSelected(new Set());
      await reload();
      onChanged();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setAdding(false);
    }
  }

  async function handleRemove(leadId: string) {
    try {
      await api.removeCampaignLead(campaign.id, leadId, tenant);
      toast.push("Lead retirado", "success");
      await reload();
      onChanged();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label="Cerrar" />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">Leads de la campaña</p>
            <h2>{campaign.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            Cerrar
          </button>
        </header>
        <div className="drawer-body">
          <section style={{ marginBottom: 24 }}>
            <h3 style={{ marginBottom: 12 }}>En la campaña ({leads.length})</h3>
            {loading ? (
              <p className="subtle">Cargando…</p>
            ) : leads.length === 0 ? (
              <p className="subtle">Aún no hay leads en esta campaña.</p>
            ) : (
              <table className="compact">
                <thead>
                  <tr>
                    <th>Nombre</th>
                    <th>Teléfono</th>
                    <th>Estado</th>
                    <th>Intentos</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {leads.map((cl) => (
                    <tr key={cl.id}>
                      <td>{cl.leadName || "—"}</td>
                      <td>
                        <code className="mono">{cl.leadPhone || ""}</code>
                      </td>
                      <td>
                        <span className={statusClass(cl.status)}>{cl.status}</span>
                      </td>
                      <td>{cl.attempts}</td>
                      <td>
                        <button className="button ghost compact" onClick={() => handleRemove(cl.leadId)}>
                          Retirar
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>

          <section>
            <h3 style={{ marginBottom: 12 }}>Añadir leads existentes ({available.length} disponibles)</h3>
            {available.length === 0 ? (
              <p className="subtle">No hay más leads en este tenant para añadir.</p>
            ) : (
              <>
                <div style={{ maxHeight: 280, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 8 }}>
                  <table className="compact">
                    <thead>
                      <tr>
                        <th style={{ width: 40 }}></th>
                        <th>Nombre</th>
                        <th>Teléfono</th>
                        <th>Tipo</th>
                      </tr>
                    </thead>
                    <tbody>
                      {available.map((l) => (
                        <tr key={l.id}>
                          <td>
                            <input
                              type="checkbox"
                              checked={selected.has(l.id)}
                              onChange={(e) => {
                                const next = new Set(selected);
                                if (e.target.checked) next.add(l.id);
                                else next.delete(l.id);
                                setSelected(next);
                              }}
                            />
                          </td>
                          <td>{l.name}</td>
                          <td>
                            <code className="mono">{l.phone}</code>
                          </td>
                          <td>{l.type}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="actions" style={{ marginTop: 12 }}>
                  <button className="button" disabled={adding || selected.size === 0} onClick={handleAdd}>
                    Añadir {selected.size > 0 ? `(${selected.size})` : ""}
                  </button>
                </div>
              </>
            )}
          </section>
        </div>
      </aside>
    </div>
  );
}
