"use client";

import { useState } from "react";
import { Home, Pencil, Plus, Trash2 } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { CardGridSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import { api, ApiError, Property } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useResource } from "../../../lib/use-resource";
import { useT } from "../../../lib/i18n";

type EditState = { property: Property | null; mode: "edit" | "create" } | null;

export default function PropertiesPage() {
  const tenant = useTenantScope();
  const t = useT();
  const properties = useResource(() => api.properties(tenant), [tenant]);
  const toast = useToast();
  const confirm = useConfirm();
  const [editor, setEditor] = useState<EditState>(null);

  async function handleDelete(property: Property) {
    const ok = await confirm({
      title: t("btn.delete"),
      description: t("properties.toast.delete_confirm", { name: property.name }),
      variant: "danger",
      confirmLabel: t("btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteProperty(property.id, tenant);
      toast.push(t("properties.toast.deleted"), "success");
      properties.reload();
    } catch (err) {
      toast.push(t("properties.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("properties.title")}</h1>
          <p className="subtle">{t("properties.subtitle.full")}</p>
        </div>
        <div className="actions">
          <button className="button" onClick={() => setEditor({ property: null, mode: "create" })}>
            <Plus aria-hidden="true" />
            <span>{t("properties.btn.new")}</span>
          </button>
        </div>
      </div>

      {properties.loading ? <CardGridSkeleton count={2} /> : null}
      {properties.error ? <div className="empty-state danger">{t("g.error")}: {properties.error}</div> : null}

      {!properties.loading && !properties.error && (properties.data?.length ?? 0) === 0 ? (
        <EmptyState
          icon={Home}
          title={t("properties.empty")}
          description={t("properties.empty.desc")}
          action={{ label: t("properties.btn.new"), onClick: () => setEditor({ property: null, mode: "create" }) }}
        />
      ) : null}

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
                <h3>{t("properties.requirements")}</h3>
                <p className="subtle">{property.requirements.join(", ") || "—"}</p>
              </div>
              <div>
                <h3>{t("properties.faqs")}</h3>
                <p className="subtle">{property.faqs.join(", ") || "—"}</p>
              </div>
            </div>
            <div className="actions" style={{ marginTop: 14, justifyContent: "flex-start" }}>
              <button
                className="button secondary compact"
                onClick={() => setEditor({ property, mode: "edit" })}
              >
                <Pencil aria-hidden="true" />
                <span>{t("properties.btn.edit")}</span>
              </button>
              <button className="button ghost compact" onClick={() => handleDelete(property)}>
                <Trash2 aria-hidden="true" />
                <span>{t("properties.btn.delete")}</span>
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
  const t = useT();
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
      toast.push(t("properties.toast.warn.required"), "warn");
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
      toast.push(mode === "create" ? t("properties.toast.created") : t("properties.toast.updated"), "success");
      onSaved();
    } catch (err) {
      toast.push(t("properties.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label={t("btn.close")} />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{mode === "create" ? t("properties.editor.create.eyebrow") : t("properties.editor.edit.eyebrow")}</p>
            <h2>{mode === "create" ? t("properties.editor.create.title") : property?.name}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            {t("btn.close")}
          </button>
        </header>

        <form className="drawer-body" onSubmit={handleSubmit}>
          <div className="field">
            <label>{t("properties.editor.field.name")}</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="field">
            <label>{t("properties.editor.field.address")}</label>
            <input value={address} onChange={(e) => setAddress(e.target.value)} required />
          </div>
          <div className="form-grid">
            <div className="field">
              <label>{t("properties.editor.field.price")}</label>
              <input
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                placeholder={t("properties.editor.field.price.placeholder")}
              />
            </div>
            <div className="field">
              <label>{t("properties.editor.field.availability")}</label>
              <input
                value={availability}
                onChange={(e) => setAvailability(e.target.value)}
                placeholder={t("properties.editor.field.availability.placeholder")}
              />
            </div>
          </div>
          <div className="field">
            <label>{t("properties.editor.field.requirements")}</label>
            <textarea
              value={requirements}
              onChange={(e) => setRequirements(e.target.value)}
              placeholder={t("properties.editor.field.requirements.placeholder")}
            />
          </div>
          <div className="field">
            <label>{t("properties.editor.field.faqs")}</label>
            <textarea
              value={faqs}
              onChange={(e) => setFaqs(e.target.value)}
              placeholder={t("properties.editor.field.faqs.placeholder")}
            />
          </div>
          <button className="button" disabled={submitting}>
            {submitting
              ? t("properties.editor.submit.creating")
              : mode === "create"
              ? t("properties.editor.submit.create")
              : t("properties.editor.submit.save")}
          </button>
        </form>
      </aside>
    </div>
  );
}
