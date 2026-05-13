"use client";

import { useEffect, useState } from "react";
import { Pause, Play, Trash2, Users } from "lucide-react";
import { useToast } from "../../../components/toast";
import { api, ApiError, Bot, Campaign, CampaignLead, Lead, statusClass } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

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

  async function handleToggleStatus(c: Campaign) {
    const next = c.status === "scheduled" ? "paused" : "scheduled";
    try {
      await api.updateCampaign(c.id, { status: next }, tenant);
      toast.push(`Campaña ${next === "scheduled" ? "activada" : "pausada"}`, "success");
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
          <h1>Campanas</h1>
          <p className="subtle">Programacion, cadencia, volumen y control de llamadas por bot.</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setFormOpen((v) => !v)}>
            {formOpen ? "Cancelar" : "Programar campana"}
          </button>
        </div>
      </div>

      {formOpen ? <NewCampaignForm bots={bots.data ?? []} onSubmit={handleCreate} /> : null}

      <div className="grid two" style={{ marginBottom: 16 }}>
        {(campaigns.data ?? []).map((campaign) => (
          <section className="panel" key={campaign.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">Campana</p>
                <h2>{campaign.name}</h2>
              </div>
              <span className={statusClass(campaign.status)}>{campaign.status}</span>
            </div>
            <div className="command-strip">
              <div className="command-row">
                <span>Horario</span>
                <strong>{campaign.schedule || "—"}</strong>
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
                <span>Intentos máximos</span>
                <strong>{campaign.maxAttempts}</strong>
              </div>
            </div>
            <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
              <button className="button compact" onClick={() => setLeadsDrawer(campaign)}>
                <Users aria-hidden="true" />
                <span>Gestionar leads</span>
              </button>
              <button className="button secondary compact" onClick={() => handleToggleStatus(campaign)}>
                {campaign.status === "scheduled" ? (
                  <>
                    <Pause aria-hidden="true" />
                    <span>Pausar</span>
                  </>
                ) : (
                  <>
                    <Play aria-hidden="true" />
                    <span>Activar</span>
                  </>
                )}
              </button>
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
            <h2>Reglas antes de llamar</h2>
          </div>
          <span className="status warn">Requiere validacion</span>
        </div>
        <div className="grid three">
          <div>
            <h3>Consentimiento</h3>
            <p className="subtle">Cruzar cada lead con fuente, opt-out y base de contacto.</p>
          </div>
          <div>
            <h3>Horario</h3>
            <p className="subtle">Respetar zona horaria del tenant y ventanas configuradas.</p>
          </div>
          <div>
            <h3>Volumen</h3>
            <p className="subtle">Limites diarios por cliente, campana y numero de salida.</p>
          </div>
        </div>
      </section>
    </>
  );
}

function NewCampaignForm({ bots, onSubmit }: { bots: Bot[]; onSubmit: (input: Partial<Campaign>) => Promise<void> }) {
  const [name, setName] = useState("");
  const [botId, setBotId] = useState(bots[0]?.id ?? "");
  const [schedule, setSchedule] = useState("Weekdays 10:00-18:00");
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
        await onSubmit({ name, botId, schedule, maxAttempts, status });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Nueva campana</p>
          <h2>Programar</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Nombre</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="field">
          <label>Bot</label>
          <select value={botId} onChange={(e) => setBotId(e.target.value)}>
            <option value="">—</option>
            {bots.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>Horario</label>
          <input value={schedule} onChange={(e) => setSchedule(e.target.value)} />
        </div>
        <div className="field">
          <label>Intentos maximos</label>
          <input
            type="number"
            min={1}
            max={10}
            value={maxAttempts}
            onChange={(e) => setMaxAttempts(parseInt(e.target.value, 10) || 1)}
          />
        </div>
        <div className="field">
          <label>Estado</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="draft">Draft</option>
            <option value="scheduled">Scheduled</option>
            <option value="paused">Paused</option>
          </select>
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : "Crear campana"}
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
  const toast = useToast();
  const members = useResource(() => api.campaignLeads(campaign.id, tenant), [campaign.id, tenant]);
  const allLeads = useResource(() => api.leads(tenant), [tenant]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const memberIds = new Set((members.data ?? []).map((m) => m.leadId));
  const available = (allLeads.data ?? []).filter(
    (l) =>
      !memberIds.has(l.id) &&
      (query.trim() === "" ||
        l.name.toLowerCase().includes(query.toLowerCase()) ||
        l.phone.includes(query)),
  );

  useEffect(() => {
    setSelected(new Set());
  }, [campaign.id]);

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleAdd() {
    if (selected.size === 0) return;
    setSubmitting(true);
    try {
      const res = await api.addCampaignLeads(campaign.id, Array.from(selected), tenant);
      toast.push(`Añadidos ${res.created} (total ${res.total})`, "success");
      setSelected(new Set());
      members.reload();
      onChanged();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRemove(leadId: string) {
    try {
      await api.removeCampaignLead(campaign.id, leadId, tenant);
      toast.push("Lead retirado", "success");
      members.reload();
      onChanged();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label="Cerrar" />
      <aside className="drawer" style={{ width: "min(640px, 100%)" }}>
        <header className="drawer-header">
          <div>
            <p className="eyebrow">Campaña · {campaign.name}</p>
            <h2>Leads asignados</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            Cerrar
          </button>
        </header>

        <div className="drawer-body">
          <p className="subtle">
            Status: <strong>{campaign.status}</strong> · Intentos máx: <strong>{campaign.maxAttempts}</strong> ·
            Cooldown: <strong>{campaign.retryCooldownMinutes} min</strong>
          </p>

          <h3 style={{ marginTop: 12 }}>Miembros ({(members.data ?? []).length})</h3>
          {members.loading ? (
            <div className="empty-state">Cargando…</div>
          ) : (members.data ?? []).length === 0 ? (
            <div className="empty-state">Campaña sin leads aún. Añade abajo desde el listado.</div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Lead</th>
                    <th>Teléfono</th>
                    <th>Status</th>
                    <th>Intentos</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {(members.data ?? []).map((m: CampaignLead) => (
                    <tr key={m.id}>
                      <td className="primary-cell">{m.leadName}</td>
                      <td>{m.leadPhone}</td>
                      <td>
                        <span className={statusClass(m.status)}>{m.status}</span>
                      </td>
                      <td>{m.attempts}</td>
                      <td>
                        <button className="button ghost compact" onClick={() => handleRemove(m.leadId)}>
                          <Trash2 aria-hidden="true" />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <h3 style={{ marginTop: 16 }}>Añadir desde mis leads</h3>
          <input
            className="search-input"
            placeholder="Buscar nombre o teléfono…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <div className="table-wrap" style={{ maxHeight: 320 }}>
            {allLeads.loading ? (
              <div className="empty-state">Cargando leads…</div>
            ) : available.length === 0 ? (
              <div className="empty-state">No hay leads disponibles para añadir.</div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th></th>
                    <th>Nombre</th>
                    <th>Teléfono</th>
                    <th>Estado</th>
                  </tr>
                </thead>
                <tbody>
                  {available.map((l: Lead) => (
                    <tr key={l.id} onClick={() => toggle(l.id)} style={{ cursor: "pointer" }}>
                      <td>
                        <input type="checkbox" checked={selected.has(l.id)} readOnly />
                      </td>
                      <td className="primary-cell">{l.name}</td>
                      <td>{l.phone}</td>
                      <td>
                        <span className={statusClass(l.status)}>{l.status}</span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          <button className="button" disabled={submitting || selected.size === 0} onClick={handleAdd}>
            {submitting ? "Añadiendo…" : `Añadir ${selected.size} lead${selected.size === 1 ? "" : "s"}`}
          </button>
        </div>
      </aside>
    </div>
  );
}
