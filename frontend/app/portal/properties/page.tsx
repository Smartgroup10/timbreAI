"use client";

import { useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useToast } from "../../../components/toast";
import { api, ApiError, Property } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";

type EditState = { property: Property | null; mode: "edit" | "create" } | null;

export default function PropertiesPage() {
  const tenant = useTenantScope();
  const properties = useResource(() => api.properties(tenant), [tenant]);
  const toast = useToast();
  const [editor, setEditor] = useState<EditState>(null);

  async function handleDelete(property: Property) {
    if (!confirm(`Eliminar la propiedad "${property.name}"?`)) return;
    try {
      await api.deleteProperty(property.id, tenant);
      toast.push("Propiedad eliminada", "success");
      properties.reload();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Propiedades</h1>
          <p className="subtle">Conocimiento verificado que el bot puede usar en llamada sin inventar condiciones.</p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setEditor({ property: null, mode: "create" })}>
            <Plus aria-hidden="true" />
            <span>Nueva propiedad</span>
          </button>
        </div>
      </div>

      {properties.loading ? <div className="empty-state">Cargando…</div> : null}
      {properties.error ? <div className="empty-state danger">Error: {properties.error}</div> : null}

      <div className="grid two">
        {(properties.data ?? []).map((property) => (
          <section className="panel" key={property.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">{property.address}</p>
                <h2>{property.name}</h2>
              </div>
              <span className="status good">{property.availability}</span>
            </div>
            <p>
              <strong>{property.price}</strong>
            </p>
            <div className="command-strip">
              <div>
                <h3>Requisitos</h3>
                <p className="subtle">{property.requirements.join(", ") || "—"}</p>
              </div>
              <div>
                <h3>FAQs</h3>
                <p className="subtle">{property.faqs.join(", ") || "—"}</p>
              </div>
            </div>
            <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
              <button
                className="button secondary compact"
                onClick={() => setEditor({ property, mode: "edit" })}
              >
                <Pencil aria-hidden="true" />
                <span>Editar</span>
              </button>
              <button className="button ghost compact" onClick={() => handleDelete(property)}>
                <Trash2 aria-hidden="true" />
                <span>Eliminar</span>
              </button>
            </div>
          </section>
        ))}
      </div>

      {editor ? (
        <PropertyEditor
          property={editor.property}
          mode={editor.mode}
          onClose={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            properties.reload();
          }}
        />
      ) : null}
    </>
  );
}

function PropertyEditor({
  property,
  mode,
  onClose,
  onSaved,
}: {
  property: Property | null;
  mode: "edit" | "create";
  onClose: () => void;
  onSaved: () => void;
}) {
  const tenant = useTenantScope();
  const toast = useToast();
  const [name, setName] = useState(property?.name ?? "");
  const [address, setAddress] = useState(property?.address ?? "");
  const [price, setPrice] = useState(property?.price ?? "");
  const [availability, setAvailability] = useState(property?.availability ?? "");
  const [requirements, setRequirements] = useState((property?.requirements ?? []).join("\n"));
  const [faqs, setFaqs] = useState((property?.faqs ?? []).join("\n"));
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!name.trim() || !address.trim()) {
      toast.push("Nombre y dirección requeridos", "warn");
      return;
    }
    setSubmitting(true);
    try {
      const payload = {
        name,
        address,
        price,
        availability,
        requirements: requirements.split("\n").map((s) => s.trim()).filter(Boolean),
        faqs: faqs.split("\n").map((s) => s.trim()).filter(Boolean),
      };
      if (mode === "create") {
        await api.createProperty(payload, tenant);
      } else if (property) {
        await api.updateProperty(property.id, payload, tenant);
      }
      toast.push(mode === "create" ? "Propiedad creada" : "Propiedad actualizada", "success");
      onSaved();
    } catch (err) {
      toast.push(`Error: ${err instanceof ApiError ? err.code : "error"}`, "danger");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label="Cerrar" />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{mode === "create" ? "Nueva propiedad" : "Editar propiedad"}</p>
            <h2>{mode === "create" ? "Añadir" : property?.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            Cerrar
          </button>
        </header>

        <form className="drawer-body" onSubmit={handleSubmit}>
          <div className="field">
            <label>Nombre</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="field">
            <label>Dirección</label>
            <input value={address} onChange={(e) => setAddress(e.target.value)} required />
          </div>
          <div className="form-grid">
            <div className="field">
              <label>Precio</label>
              <input value={price} onChange={(e) => setPrice(e.target.value)} placeholder="€1,200/mes" />
            </div>
            <div className="field">
              <label>Disponibilidad</label>
              <input value={availability} onChange={(e) => setAvailability(e.target.value)} placeholder="Inmediata" />
            </div>
          </div>
          <div className="field">
            <label>Requisitos (uno por línea)</label>
            <textarea
              value={requirements}
              onChange={(e) => setRequirements(e.target.value)}
              placeholder={"Verificación de ingresos\nNo desahucios"}
            />
          </div>
          <div className="field">
            <label>FAQs (una por línea)</label>
            <textarea
              value={faqs}
              onChange={(e) => setFaqs(e.target.value)}
              placeholder={"Mascotas con depósito\nGastos comunes incluidos"}
            />
          </div>
          <button className="button" disabled={submitting}>
            {submitting ? "Guardando…" : mode === "create" ? "Crear propiedad" : "Guardar cambios"}
          </button>
        </form>
      </aside>
    </div>
  );
}
