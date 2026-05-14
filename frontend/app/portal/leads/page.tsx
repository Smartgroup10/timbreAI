"use client";

import { useMemo, useRef, useState } from "react";
import Link from "next/link";
import { PhoneCall, Trash2, Upload, Users } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { TableSkeleton } from "../../../components/skeleton";
import { TestCallDrawer } from "../../../components/test-call-drawer";
import { useToast } from "../../../components/toast";
import { api, ApiError, ImportResult, Lead } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT } from "../../../lib/i18n";

const TYPE_FILTERS = ["all", "renter", "owner"] as const;
const STATUS_FILTERS = ["all", "new", "qualified", "callback", "do_not_call"] as const;
const LEAD_STATUSES = ["new", "qualified", "callback", "contacted", "do_not_call"];

export default function LeadsPage() {
  const tenant = useTenantScope();
  const t = useT();
  const leads = useResource(() => api.leads(tenant), [tenant], { pollMs: 30_000 });
  const toast = useToast();
  const confirm = useConfirm();
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
      toast.push(
        t("leads.toast.import_summary", { created: res.created, skipped: res.skipped, invalid: res.invalid }),
        res.created > 0 ? "success" : "warn"
      );
      leads.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("leads.toast.import_failed", { err: code }), "danger");
    } finally {
      setImporting(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function handleCreate(input: Partial<Lead>) {
    try {
      await api.createLead(input, tenant);
      toast.push(t("leads.toast.created"), "success");
      setFormOpen(false);
      leads.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("leads.toast.create_failed", { err: code }), "danger");
    }
  }

  async function handleStatusChange(lead: Lead, status: string) {
    try {
      await api.updateLead(lead.id, { status }, tenant);
      toast.push(t("leads.toast.status_updated"), "success");
      leads.reload();
    } catch (err) {
      toast.push(t("leads.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  async function handleDelete(lead: Lead) {
    const ok = await confirm({
      title: t("btn.delete"),
      description: t("leads.toast.delete_confirm", { name: lead.name }),
      variant: "danger",
      confirmLabel: t("btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteLead(lead.id, tenant);
      toast.push(t("leads.toast.deleted"), "success");
      leads.reload();
    } catch (err) {
      toast.push(t("leads.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("leads.title")}</h1>
          <p className="subtle">{t("leads.subtitle.full")}</p>
        </div>
        <div className="actions">
          <button className="button secondary" onClick={() => setFormOpen((v) => !v)}>
            {formOpen ? t("leads.btn.cancel") : t("leads.new")}
          </button>
          <button className="button" disabled={importing} onClick={() => fileRef.current?.click()}>
            <Upload aria-hidden="true" />
            <span>{importing ? t("leads.btn.importing") : t("leads.import")}</span>
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
          <p className="eyebrow">{t("leads.import.eyebrow")}</p>
          <h2>
            {t("leads.import.summary", {
              created: importResult.created,
              skipped: importResult.skipped,
              invalid: importResult.invalid,
            })}
          </h2>
          {importResult.errors && importResult.errors.length > 0 ? (
            <details>
              <summary className="subtle">{t("leads.import.errors", { n: importResult.errors.length })}</summary>
              <pre className="code-block">{importResult.errors.join("\n")}</pre>
            </details>
          ) : null}
          <p className="subtle">
            {t("leads.import.format", {
              cols: "name,phone,email,type,source,consent",
              req: "name, phone",
            })}
          </p>
          <button className="button ghost compact" onClick={() => setImportResult(null)}>
            {t("btn.close")}
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
              {opt === "all" ? t("leads.filter.alltypes") : opt}
            </button>
          ))}
          <span className="filter-sep" />
          {STATUS_FILTERS.map((opt) => (
            <button
              key={opt}
              className={`chip-button${statusFilter === opt ? " active" : ""}`}
              onClick={() => setStatusFilter(opt)}
            >
              {opt === "all" ? t("leads.filter.anystatus") : opt}
            </button>
          ))}
        </div>
        <input
          className="search-input"
          placeholder={t("leads.search.placeholder")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      {leads.loading ? (
        <TableSkeleton cols={8} rows={6} />
      ) : leads.error ? (
        <div className="empty-state danger">{t("g.error")}: {leads.error}</div>
      ) : (leads.data?.length ?? 0) === 0 ? (
        <EmptyState
          icon={Users}
          title={t("leads.empty")}
          description={t("leads.empty.desc")}
          action={{ label: t("leads.new"), onClick: () => setFormOpen(true) }}
          secondary={{ label: t("leads.import"), onClick: () => fileRef.current?.click() }}
        />
      ) : filtered.length === 0 ? (
        <EmptyState title={t("leads.empty.nomatch")} />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("col.name")}</th>
                <th>{t("col.phone")}</th>
                <th>{t("leads.col.email")}</th>
                <th>{t("col.type")}</th>
                <th>{t("col.status")}</th>
                <th>{t("leads.col.source")}</th>
                <th>{t("leads.col.consent")}</th>
                <th>{t("leads.col.action")}</th>
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
                      <span>{t("leads.actions.call")}</span>
                    </button>
                    <button className="button ghost compact" onClick={() => handleDelete(lead)}>
                      <Trash2 aria-hidden="true" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

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
  const t = useT();
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
          <p className="eyebrow">{t("leads.form.eyebrow")}</p>
          <h2>{t("leads.form.title")}</h2>
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label>{t("col.name")}</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="field">
          <label>{t("col.phone")}</label>
          <input value={phone} onChange={(e) => setPhone(e.target.value)} required />
        </div>
        <div className="field">
          <label>{t("leads.col.email")}</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="field">
          <label>{t("col.type")}</label>
          <select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="renter">Renter</option>
            <option value="owner">Owner</option>
          </select>
        </div>
        <div className="field">
          <label>{t("leads.col.source")}</label>
          <input value={source} onChange={(e) => setSource(e.target.value)} />
        </div>
        <div className="field">
          <label>{t("leads.col.consent")}</label>
          <input value={consent} onChange={(e) => setConsent(e.target.value)} />
        </div>
      </div>
      <div className="actions" style={{ marginTop: 12 }}>
        <button className="button" disabled={submitting}>
          {submitting ? t("leads.form.submitting") : t("leads.form.submit")}
        </button>
      </div>
    </form>
  );
}
