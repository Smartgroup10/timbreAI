"use client";

import { useState } from "react";
import { PhoneOff } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { TableSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import { api, ApiError } from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useRealtime } from "../../../lib/use-realtime";
import { useResource } from "../../../lib/use-resource";
import { useT } from "../../../lib/i18n";

const POLL_MS = 15_000;

export default function DoNotCallPage() {
  const tenant = useTenantScope();
  const t = useT();
  const dnc = useResource(() => api.dnc(tenant), [tenant], { pollMs: POLL_MS });

  // Realtime: el polling de 15s sigue ahí como red de seguridad, pero
  // un add/remove de cualquier otro operador del mismo tenant llega al
  // instante por WS.
  useRealtime((ev) => {
    if (ev.type === "dnc.changed") dnc.reload();
  });
  const toast = useToast();
  const confirm = useConfirm();
  const [phone, setPhone] = useState("");
  const [reason, setReason] = useState("opt_out");
  const [submitting, setSubmitting] = useState(false);

  async function handleAdd(event: React.FormEvent) {
    event.preventDefault();
    if (!phone.trim()) return;
    setSubmitting(true);
    try {
      await api.addDNC({ phone: phone.trim(), reason }, tenant);
      toast.push(t("dnc.toast.added"), "success");
      setPhone("");
      dnc.reload();
    } catch (err) {
      toast.push(t("dnc.toast.add_failed", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRemove(id: string, phoneNumber: string) {
    const ok = await confirm({
      title: t("dnc.table.release"),
      description: t("dnc.toast.remove_confirm", { phone: phoneNumber }),
      confirmLabel: t("dnc.table.release"),
    });
    if (!ok) return;
    try {
      await api.removeDNC(id, tenant);
      toast.push(t("dnc.toast.removed"), "success");
      dnc.reload();
    } catch (err) {
      toast.push(t("dnc.toast.error", { err: err instanceof ApiError ? err.code : "error" }), "danger");
    }
  }

  const entries = dnc.data ?? [];

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("portal.eyebrow")}</p>
          <h1>{t("dnc.title")}</h1>
          <p className="subtle">{t("dnc.subtitle.full")}</p>
        </div>
        <div className="actions">
          <span className="refresh-dot" aria-live="polite">
            {t("empty.live", { n: POLL_MS / 1000 })}
          </span>
        </div>
      </div>

      <form className="panel" style={{ marginBottom: 16 }} onSubmit={handleAdd}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("dnc.form.eyebrow")}</p>
            <h2>{t("dnc.form.title")}</h2>
          </div>
        </div>
        <div className="form-grid">
          <div className="field">
            <label>{t("dnc.form.phone")}</label>
            <input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+34600123456" required />
          </div>
          <div className="field">
            <label>{t("dnc.form.reason")}</label>
            <select value={reason} onChange={(e) => setReason(e.target.value)}>
              <option value="opt_out">{t("dnc.form.reason.opt_out")}</option>
              <option value="complaint">{t("dnc.form.reason.complaint")}</option>
              <option value="legal">{t("dnc.form.reason.legal")}</option>
              <option value="manual">{t("dnc.form.reason.manual")}</option>
            </select>
          </div>
        </div>
        <div className="actions" style={{ marginTop: 12 }}>
          <button className="button" disabled={submitting}>
            {submitting ? t("dnc.form.submitting") : t("dnc.form.submit")}
          </button>
        </div>
      </form>

      {dnc.loading ? (
        <TableSkeleton cols={4} rows={4} />
      ) : entries.length === 0 ? (
        <EmptyState
          icon={PhoneOff}
          title={t("dnc.table.empty")}
          description={t("dnc.empty.desc")}
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("dnc.field.phone")}</th>
                <th>{t("dnc.field.reason")}</th>
                <th>{t("dnc.field.added")}</th>
                <th>{t("dnc.table.action")}</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr key={e.id}>
                  <td className="primary-cell">
                    <code className="mono">{e.phone}</code>
                  </td>
                  <td>
                    <span className="chip">{e.reason}</span>
                  </td>
                  <td>{new Date(e.createdAt).toLocaleString()}</td>
                  <td>
                    <button className="button ghost compact" onClick={() => handleRemove(e.id, e.phone)}>
                      {t("dnc.table.release")}
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
