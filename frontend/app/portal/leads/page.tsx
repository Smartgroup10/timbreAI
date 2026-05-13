"use client";

import { useMemo, useRef, useState } from "react";
import Link from "next/link";
import { PhoneCall, Trash2, Upload } from "lucide-react";
import { TestCallDrawer } from "../../../components/test-call-drawer";
import { useToast } from "../../../components/toast";
import { api, ApiError, ImportResult, Lead, statusClass } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

const TYPE_FILTERS = ["all", "renter", "owner"] as const;
const STATUS_FILTERS = ["all", "new", "qualified", "callback", "do_not_call"] as const;
const LEAD_STATUSES = ["new", "qualified", "callback", "contacted", "do_not_call"];

export default function LeadsPage() {
  const tenant = useTenantScope();
  const leads = useResource(() => api.leads(tenant), [tenant]);
  const toast = useToast();
  const [formOpen, setFormOpen] = useState(false);
  const [drawerLead, setDrawerLead] = useState<Lead | null>(null);
  const [typeFilter, setTypeFilter] = useState<(typeof TYPE_FILTERS)[number]>("all");
  const [statusFilter, setStatusFilter] = useState<(typeof STATUS_FILTERS)[number]>("all");
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const data = leads.data ?? [];
    return data.filter((l) => {
      if (typeFilter !== "all" && l.type !== typeFilter) return false;
      if (statusFilter !== "all" && l.status !== statusFilter) return false;
      if (query.trim()) {
        const q = query.toLowerCase();
        if (!l.name.toLowerCase().includes(q) && !l.phone.includes(q) && !l.email.toLowerCase().includes(q)) {
          return false;
        }
      }
      return true;
    });
  }, [leads.data, typeFilter, statusFilter, query]);

  const fileRef = useRef<HTMLInputElement>(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);

  async function handleImportFile(file: File) {
    setImporting(true);
    setImportResult(null);
    try {
      const csv = await file.text();
      const res = await api.importLeads(csv, tenant);
      setImportResult(res);
      toast.push(`Importados ${res.created} · ${res.skipped} omitidos · ${res.invalid} inválidos`, res.created > 0 ? "success" : "warn");
      leads.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(`Error importando: ${code}`, "danger");
    } finally {
      setImporting(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function handleCreate(input: Partial<Lead>) {
    try {
      await api.createLead(input, tenant);
      toast.push("Lead creado", "success");
      setFormOpen(false);
      leads.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(`No se pudo crear: ${code}`, "danger");
    }
  }

  async function handleStatusChange(lead: Lead, status: string) {
    try {
      await api.updateLead(lead.id, { status }, tenant);
      toast.push("Estado actualizado", "success");
      leads.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  async function handleDelete(lead: Lead) {
    if (!confirm(`Eliminar el lead "${lead.name}"?`)) return;
    try {
      await api.deleteLead(lead.id, tenant);
      toast.push("Lead eliminado", "success");
      leads.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Leads</h1>
          <p className="subtle">Contactos disponibles para campanas, llamadas de seguimiento y handoff comercial.</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setFormOpen((v) => !v)}>
            {formOpen ? "Cancelar" : "Nuevo lead"}
          </button>
          <button className="button" disabled={importing} onClick={() => fileRef.current?.click()}>
            <Upload aria-hidden="true" />
            <span>{importing ? "Importando…" : "Importar CSV"}</span>
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,text/csv"
            style={{ display: "none" }}
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) handleImportFile(file);
            }}
          />
        </div>
      </div>

      {importResult ? (
        <div className="panel" style={{ marginBottom: 12 }}>
          <p className="eyebrow">Resultado de importación</p>
          <h2>
            {importResult.created} creados · {importResult.skipped} omitidos · {importResult.invalid} inválidos
          </h2>
          {importResult.errors && importResult.errors.length > 0 ? (
            <details>
              <summary className="subtle">{importResult.errors.length} errores</summary>
              <pre className="code-block">{importResult.errors.join("\n")}</pre>
            </details>
          ) : null}
          <p className="subtle">
            Formato: cabecera con columnas <code>name,phone,email,type,source,consent</code>. Solo
            <code> name</code> y <code>phone</code> son obligatorios. Los teléfonos en DNC o duplicados
            se omiten.
          </p>
          <button className="button ghost compact" onClick={() => setImportResult(null)}>
            Cerrar
          </button>
        </div>
      ) : null}

      {formOpen ? <NewLeadForm onSubmit={handleCreate} /> : null}

      <div className="toolbar">
        <div className="filter-row">
          {TYPE_FILTERS.map((opt) => (
            <button
              key={opt}
              className={`chip-button${typeFilter === opt ? " active" : ""}`}
              onClick={() => setTypeFilter(opt)}
            >
              {opt === "all" ? "Todos" : opt}
            </button>
          ))}
          <span className="filter-sep" />
          {STATUS_FILTERS.map((opt) => (
            <button
              key={opt}
              className={`chip-button${statusFilter === opt ? " active" : ""}`}
              onClick={() => setStatusFilter(opt)}
            >
              {opt === "all" ? "Cualquier estado" : opt}
            </button>
          ))}
        </div>
        <input
          className="search-input"
          placeholder="Buscar por nombre, telefono o email…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      <div className="table-wrap">
        {leads.loading ? (
          <div className="empty-state">Cargando leads…</div>
        ) : leads.error ? (
          <div className="empty-state danger">Error: {leads.error}</div>
        ) : filtered.length === 0 ? (
          <div className="empty-state">Sin leads que coincidan con el filtro.</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Nombre</th>
                <th>Telefono</th>
                <th>Email</th>
                <th>Tipo</th>
                <th>Estado</th>
                <th>Fuente</th>
                <th>Consentimiento</th>
                <th>Accion</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((lead) => (
                <tr key={lead.id}>
                  <td className="primary-cell">
                    <Link href={`/portal/leads/${lead.id}`} style={{ color: "inherit" }}>
                      {lead.name}
                    </Link>
                  </td>
                  <td>{lead.phone}</td>
                  <td>{lead.email || "—"}</td>
                  <td>
                    <span className="chip">{lead.type}</span>
                  </td>
                  <td>
                    <select
                      className="inline-select"
                      value={lead.status}
                      onChange={(e) => handleStatusChange(lead, e.target.value)}
                    >
                      {LEAD_STATUSES.map((s) => (
                        <option key={s} value={s}>
                          {s}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td>{lead.source}</td>
                  <td>{lead.consent}</td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    <button className="button ghost compact" onClick={() => setDrawerLead(lead)}>
                      <PhoneCall aria-hidden="true" />
                      <span>Llamar</span>
                    </button>
                    <button className="button ghost compact" onClick={() => handleDelete(lead)}>
                      <Trash2 aria-hidden="true" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <TestCallDrawer
        open={drawerLead !== null}
        onClose={() => setDrawerLead(null)}
        defaultPhone={drawerLead?.phone}
        defaultLeadName={drawerLead?.name}
      />
    </>
  );
}

function NewLeadForm({ onSubmit }: { onSubmit: (input: Partial<Lead>) => Promise<void> }) {
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [type, setType] = useState("renter");
  const [source, setSource] = useState("portal");
  const [consent, setConsent] = useState("manual");
  const [submitting, setSubmitting] = useState(false);

  return (
    <form
      className="panel"
      style={{ marginBottom: 16 }}
      onSubmit={async (event) => {
        event.preventDefault();
        setSubmitting(true);
        await onSubmit({ name, phone, email, type, source, consent });
        setSubmitting(false);
      }}
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Nuevo lead</p>
          <h2>Crear contacto</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>Nombre</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="field">
          <label>Telefono</label>
          <input value={phone} onChange={(e) => setPhone(e.target.value)} required />
        </div>
        <div className="field">
          <label>Email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="field">
          <label>Tipo</label>
          <select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="renter">Renter</option>
            <option value="owner">Owner</option>
          </select>
        </div>
        <div className="field">
          <label>Fuente</label>
          <input value={source} onChange={(e) => setSource(e.target.value)} />
        </div>
        <div className="field">
          <label>Consentimiento</label>
          <input value={consent} onChange={(e) => setConsent(e.target.value)} />
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? "Guardando…" : "Crear lead"}
        </button>
      </div>
    </form>
  );
}
