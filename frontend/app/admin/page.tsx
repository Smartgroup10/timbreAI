"use client";

import { useState } from "react";
import { Building2, Plus, Pencil } from "lucide-react";
import { EmptyState } from "../../components/empty";
import { TableSkeleton } from "../../components/skeleton";
import { useToast } from "../../components/toast";
import { api, ApiError, Tenant, statusClass } from "../../lib/api";
import { useAuth } from "../../lib/auth-context";
import { useResource } from "../../lib/use-resource";
import { useT, useStatusLabel } from "../../lib/i18n";

type EditState = { mode: "create" } | { mode: "edit"; tenant: Tenant } | null;

export default function AdminPage() {
  const { user, setTenantOverride } = useAuth();
  const t = useT();
  const statusLabel = useStatusLabel();
  const tenants = useResource(() => api.tenants(), [], { pollMs: 30_000 });
  const [editor, setEditor] = useState<EditState>(null);
  const toast = useToast();

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">{t("admin.tenants.access.denied")}</div>;
  }

  const data = tenants.data ?? [];

  async function handleSave(input: { id: string; name: string; plan: string; status: string }, mode: "create" | "edit") {
    try {
      if (mode === "create") {
        await api.adminCreateTenant(input);
        toast.push(t("admin.tenants.toast.created"), "success");
      } else {
        await api.adminUpdateTenant(input.id, { name: input.name, plan: input.plan, status: input.status });
        toast.push(t("admin.tenants.toast.updated"), "success");
      }
      setEditor(null);
      tenants.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("admin.tenants.toast.error", { err: code }), "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("admin.eyebrow")}</p>
          <h1>{t("nav.tenants")}</h1>
          <p className="subtle">{t("admin.tenants.subtitle.full")}</p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setEditor({ mode: "create" })}>
            <Plus aria-hidden="true" />
            <span>{t("admin.tenants.btn.create")}</span>
          </button>
        </div>
      </div>

      <div className="grid three" style={{ marginBottom: 16 }}>
        <section className="panel">
          <p className="eyebrow">{t("admin.tenants.stat.total")}</p>
          <span className="stat-value">{data.length}</span>
          <p className="subtle">{t("admin.tenants.stat.total.hint")}</p>
        </section>
        <section className="panel">
          <p className="eyebrow">{t("admin.tenants.stat.active")}</p>
          <span className="stat-value">{data.filter((tn) => tn.status === "active").length}</span>
          <p className="subtle">{t("admin.tenants.stat.active.hint")}</p>
        </section>
        <section className="panel">
          <p className="eyebrow">{t("admin.tenants.stat.platform")}</p>
          <span className="stat-value">{data.filter((tn) => tn.plan === "platform").length}</span>
          <p className="subtle">{t("admin.tenants.stat.platform.hint")}</p>
        </section>
      </div>

      {tenants.loading ? (
        <TableSkeleton cols={6} rows={5} />
      ) : tenants.error ? (
        <div className="empty-state danger">{t("g.error")}: {tenants.error}</div>
      ) : data.length === 0 ? (
        <EmptyState
          icon={Building2}
          title={t("admin.tenants.empty")}
          description={t("admin.tenants.empty.desc")}
          action={{ label: t("admin.tenants.btn.create"), onClick: () => setEditor({ mode: "create" }) }}
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("admin.tenants.col.customer")}</th>
                <th>{t("admin.tenants.col.tenantid")}</th>
                <th>{t("admin.tenants.col.plan")}</th>
                <th>{t("admin.tenants.col.status")}</th>
                <th>{t("admin.tenants.col.created")}</th>
                <th>{t("admin.tenants.col.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {data.map((tenant) => (
                <tr key={tenant.id}>
                  <td className="primary-cell">{tenant.name}</td>
                  <td>
                    <code className="mono">{tenant.id}</code>
                  </td>
                  <td>
                    <span className="chip">{tenant.plan}</span>
                  </td>
                  <td>
                    <span className={statusClass(tenant.status)}>{statusLabel(tenant.status)}</span>
                  </td>
                  <td>{new Date(tenant.createdAt).toLocaleDateString()}</td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    <button className="button secondary compact" onClick={() => setEditor({ mode: "edit", tenant })}>
                      <Pencil aria-hidden="true" />
                      <span>{t("admin.tenants.btn.edit")}</span>
                    </button>
                    <button className="button ghost compact" onClick={() => setTenantOverride(tenant.id)}>
                      {t("admin.tenants.btn.impersonate")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editor ? (
        <TenantEditor
          mode={editor.mode}
          tenant={editor.mode === "edit" ? editor.tenant : null}
          onClose={() => setEditor(null)}
          onSave={handleSave}
        />
      ) : null}
    </>
  );
}

function TenantEditor({
  mode,
  tenant,
  onClose,
  onSave,
}: {
  mode: "create" | "edit";
  tenant: Tenant | null;
  onClose: () => void;
  onSave: (input: { id: string; name: string; plan: string; status: string }, mode: "create" | "edit") => Promise<void>;
}) {
  const t = useT();
  const [id, setId] = useState(tenant?.id ?? "");
  const [name, setName] = useState(tenant?.name ?? "");
  const [plan, setPlan] = useState(tenant?.plan ?? "starter");
  const [status, setStatus] = useState(tenant?.status ?? "active");
  const [submitting, setSubmitting] = useState(false);

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label={t("btn.close")} />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{mode === "create" ? t("admin.tenants.editor.create.eyebrow") : t("admin.tenants.editor.edit.eyebrow")}</p>
            <h2>{mode === "create" ? t("admin.tenants.editor.create.title") : tenant?.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            {t("btn.close")}
          </button>
        </header>
        <form
          className="drawer-body"
          onSubmit={async (event) => {
            event.preventDefault();
            setSubmitting(true);
            await onSave({ id, name, plan, status }, mode);
            setSubmitting(false);
          }}
        >
          <div className="field">
            <label>{t("admin.tenants.editor.id")} {mode === "create" ? <span className="subtle">{t("admin.tenants.editor.id.slug")}</span> : null}</label>
            <input value={id} onChange={(e) => setId(e.target.value)} required readOnly={mode === "edit"} pattern="^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$" />
            <p className="subtle" style={{ marginTop: 4 }}>{t("admin.tenants.editor.id.hint")}</p>
          </div>
          <div className="field">
            <label>{t("admin.tenants.editor.name")}</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="form-grid">
            <div className="field">
              <label>{t("admin.tenants.editor.plan")}</label>
              <select value={plan} onChange={(e) => setPlan(e.target.value)}>
                <option value="starter">Starter</option>
                <option value="growth">Growth</option>
                <option value="platform">Platform</option>
              </select>
            </div>
            <div className="field">
              <label>{t("admin.tenants.editor.status")}</label>
              <select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="active">{t("admin.tenants.editor.status.active")}</option>
                <option value="paused">{t("admin.tenants.editor.status.paused")}</option>
                <option value="suspended">{t("admin.tenants.editor.status.suspended")}</option>
              </select>
            </div>
          </div>
          <button className="button" disabled={submitting}>
            {submitting ? t("admin.tenants.editor.submitting") : mode === "create" ? t("admin.tenants.editor.submit.create") : t("admin.tenants.editor.submit.save")}
          </button>
        </form>
      </aside>
    </div>
  );
}
