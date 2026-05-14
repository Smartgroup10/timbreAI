"use client";

import { useState } from "react";
import { Plus, Pencil } from "lucide-react";
import { useToast } from "../../components/toast";
import { api, ApiError, Tenant, statusClass, statusLabel } from "../../lib/api";
import { useAuth } from "../../lib/auth-context";
import { useResource } from "../../lib/use-resource";

type EditState = { mode: "create" } | { mode: "edit"; tenant: Tenant } | null;

export default function AdminPage() {
  const { user, setTenantOverride } = useAuth();
  const tenants = useResource(() => api.tenants(), []);
  const [editor, setEditor] = useState<EditState>(null);
  const toast = useToast();

  if (user && user.role !== "platform_admin") {
    return <div className="empty-state danger">Acceso restringido al rol platform_admin.</div>;
  }

  const data = tenants.data ?? [];

  async function handleSave(input: { id: string; name: string; plan: string; status: string }, mode: "create" | "edit") {
    try {
      if (mode === "create") {
        await api.adminCreateTenant(input);
        toast.push("Tenant creado", "success");
      } else {
        await api.adminUpdateTenant(input.id, { name: input.name, plan: input.plan, status: input.status });
        toast.push("Tenant actualizado", "success");
      }
      setEditor(null);
      tenants.reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(`Error: ${code}`, "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Admin interno</p>
          <h1>Clientes</h1>
          <p className="subtle">Gestión multi-tenant de cuentas, límites, estado operativo y soporte.</p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setEditor({ mode: "create" })}>
            <Plus aria-hidden="true" />
            <span>Crear cliente</span>
          </button>
        </div>
      </div>

      <div className="grid three" style={{ marginBottom: 16 }}>
        <section className="panel">
          <p className="eyebrow">Tenants</p>
          <span className="stat-value">{data.length}</span>
          <p className="subtle">Cuentas visibles para admin plataforma.</p>
        </section>
        <section className="panel">
          <p className="eyebrow">Activos</p>
          <span className="stat-value">{data.filter((t) => t.status === "active").length}</span>
          <p className="subtle">Tenants en estado active.</p>
        </section>
        <section className="panel">
          <p className="eyebrow">Plan platform</p>
          <span className="stat-value">{data.filter((t) => t.plan === "platform").length}</span>
          <p className="subtle">Clientes en plan superior.</p>
        </section>
      </div>

      <div className="table-wrap">
        {tenants.loading ? (
          <div className="empty-state">Cargando…</div>
        ) : tenants.error ? (
          <div className="empty-state danger">Error: {tenants.error}</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Cliente</th>
                <th>Tenant ID</th>
                <th>Plan</th>
                <th>Estado</th>
                <th>Creado</th>
                <th>Acciones</th>
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
                      <span>Editar</span>
                    </button>
                    <button className="button ghost compact" onClick={() => setTenantOverride(tenant.id)}>
                      Suplantar
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

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
  const [id, setId] = useState(tenant?.id ?? "");
  const [name, setName] = useState(tenant?.name ?? "");
  const [plan, setPlan] = useState(tenant?.plan ?? "starter");
  const [status, setStatus] = useState(tenant?.status ?? "active");
  const [submitting, setSubmitting] = useState(false);

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label="Cerrar" />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{mode === "create" ? "Nuevo cliente" : "Editar cliente"}</p>
            <h2>{mode === "create" ? "Crear tenant" : tenant?.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            Cerrar
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
            <label>Tenant ID {mode === "create" ? <span className="subtle">(slug, sin espacios)</span> : null}</label>
            <input value={id} onChange={(e) => setId(e.target.value)} required readOnly={mode === "edit"} pattern="^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$" />
            <p className="subtle" style={{ marginTop: 4 }}>
              3-32 caracteres, minúsculas, números y guiones. Inmutable una vez creado.
            </p>
          </div>
          <div className="field">
            <label>Nombre comercial</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="form-grid">
            <div className="field">
              <label>Plan</label>
              <select value={plan} onChange={(e) => setPlan(e.target.value)}>
                <option value="starter">Starter</option>
                <option value="growth">Growth</option>
                <option value="platform">Platform</option>
              </select>
            </div>
            <div className="field">
              <label>Estado</label>
              <select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="active">Activo</option>
                <option value="paused">Pausado</option>
                <option value="suspended">Suspendido</option>
              </select>
            </div>
          </div>
          <button className="button" disabled={submitting}>
            {submitting ? "Guardando…" : mode === "create" ? "Crear cliente" : "Guardar cambios"}
          </button>
        </form>
      </aside>
    </div>
  );
}
