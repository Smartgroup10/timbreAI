"use client";

// /portal/tools — biblioteca de herramientas por tenant.
// CRUD + listado. Cada bot luego selecciona cuáles asignar en su editor.

import { useEffect, useState } from "react";
import { Pencil, Plus, Trash2, Wrench } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { TableSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import { ToolForm } from "../../../components/tool-form";
import { api, ApiError, Tool, ToolInput } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useT } from "../../../lib/i18n";

export default function ToolsPage() {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const confirm = useConfirm();
  const [tools, setTools] = useState<Tool[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Tool | null>(null);

  async function reload() {
    setLoading(true);
    try {
      const list = await api.tools(tenant);
      setTools(list);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant]);

  async function handleSave(input: ToolInput) {
    try {
      if (editing) {
        await api.updateTool(editing.id, input, tenant);
        toast.push(t("tools.toast.updated"), "success");
      } else {
        await api.createTool(input, tenant);
        toast.push(t("tools.toast.created"), "success");
      }
      setFormOpen(false);
      setEditing(null);
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  async function handleDelete(tool: Tool) {
    const ok = await confirm({
      title: t("tools.btn.delete"),
      description: t("tools.toast.delete_confirm", { name: tool.name }),
      variant: "danger",
      confirmLabel: t("tools.btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteTool(tool.id, tenant);
      toast.push(t("tools.toast.deleted"), "success");
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("tools.library.eyebrow")}</p>
          <h1>{t("tools.library.title")}</h1>
          <p className="subtle">{t("tools.library.desc")}</p>
        </div>
        <div className="actions">
          {!formOpen ? (
            <button
              type="button"
              className="button"
              onClick={() => {
                setEditing(null);
                setFormOpen(true);
              }}
            >
              <Plus aria-hidden="true" />
              <span>{t("tools.btn.add")}</span>
            </button>
          ) : null}
        </div>
      </div>

      {formOpen ? (
        <ToolForm
          initial={editing ?? undefined}
          onSubmit={handleSave}
          onCancel={() => {
            setFormOpen(false);
            setEditing(null);
          }}
        />
      ) : null}

      {loading ? (
        <TableSkeleton cols={4} rows={4} />
      ) : tools.length === 0 && !formOpen ? (
        <EmptyState
          icon={Wrench}
          title={t("tools.library.empty")}
          description={t("tools.library.empty.desc")}
          action={{
            label: t("tools.btn.add"),
            onClick: () => {
              setEditing(null);
              setFormOpen(true);
            },
          }}
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("tools.col.name")}</th>
                <th>{t("tools.col.action")}</th>
                <th>{t("tools.col.description")}</th>
                <th>{t("tools.col.status")}</th>
                <th>{t("tools.col.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {tools.map((tool) => (
                <tr key={tool.id}>
                  <td className="primary-cell">
                    <code className="mono">{tool.name}</code>
                  </td>
                  <td>
                    <span className="chip">{t(`tools.action.${tool.actionType}`)}</span>
                  </td>
                  <td className="summary-cell subtle" style={{ fontSize: 12.5 }}>
                    {tool.description}
                  </td>
                  <td>
                    <span className={`status ${tool.enabled ? "good" : "warn"}`}>
                      {tool.enabled ? t("tools.enabled") : t("tools.library.archived")}
                    </span>
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    <button
                      className="button ghost compact"
                      onClick={() => {
                        setEditing(tool);
                        setFormOpen(true);
                      }}
                      aria-label={t("tools.btn.edit")}
                    >
                      <Pencil aria-hidden="true" />
                    </button>
                    <button
                      className="button ghost compact"
                      onClick={() => handleDelete(tool)}
                      aria-label={t("tools.btn.delete")}
                      style={{ marginLeft: 4 }}
                    >
                      <Trash2 aria-hidden="true" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
