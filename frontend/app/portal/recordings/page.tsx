"use client";

// Página de grabaciones — listing paginado con filtros, audio player
// inline, descarga y borrado.
//
// Las URLs vienen pre-firmadas del backend (1h). Si el operador deja
// la pestaña abierta más de 1h y pulsa play, el navegador recibe 403
// y el player muestra error — recargar la página da URLs nuevas.

import { useEffect, useState } from "react";
import Link from "next/link";
import { Download, Mic, Trash2 } from "lucide-react";
import { useConfirm } from "../../../components/confirm";
import { EmptyState } from "../../../components/empty";
import { TableSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import {
  api,
  ApiError,
  CallRecordingListItem,
  formatBytes,
  formatDurationShort,
  statusClass,
} from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useT, useStatusLabel } from "../../../lib/i18n";

const PAGE_SIZE = 50;

const OUTCOME_FILTERS = ["", "qualified", "callback", "completed", "no_answer", "failed"];

export default function RecordingsPage() {
  const tenant = useTenantScope();
  const t = useT();
  const statusLabel = useStatusLabel();
  const toast = useToast();
  const confirm = useConfirm();
  const [items, setItems] = useState<CallRecordingListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [outcome, setOutcome] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  async function reload(targetPage = page) {
    setLoading(true);
    try {
      const r = await api.recordings(
        { page: targetPage, pageSize: PAGE_SIZE, outcome: outcome || undefined, from: from || undefined, to: to || undefined },
        tenant
      );
      setItems(r.items);
      setTotal(r.total);
      setPage(r.page);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("rec.toast.error", { err: code }), "danger");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant]);

  async function handleDelete(rec: CallRecordingListItem) {
    const ok = await confirm({
      title: t("rec.btn.delete"),
      description: t("rec.btn.delete.confirm"),
      variant: "danger",
      confirmLabel: t("rec.btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteRecording(rec.id, tenant);
      toast.push(t("rec.toast.deleted"), "success");
      // Reload manteniendo página actual; si quedó vacía, retrocede.
      const nextPage = items.length === 1 && page > 1 ? page - 1 : page;
      await reload(nextPage);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("rec.toast.error", { err: code }), "danger");
    }
  }

  function applyFilter() {
    reload(1);
  }
  function clearFilter() {
    setOutcome("");
    setFrom("");
    setTo("");
    // Reload tras siguiente render porque setState es async — simplificación:
    // disparamos reload con valores limpios.
    setTimeout(() => reload(1), 0);
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const shownFrom = items.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const shownTo = (page - 1) * PAGE_SIZE + items.length;

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("rec.eyebrow")}</p>
          <h1>{t("rec.title")}</h1>
          <p className="subtle">{t("rec.subtitle")}</p>
        </div>
      </div>

      <div className="toolbar">
        <div className="filter-row">
          {OUTCOME_FILTERS.map((o) => (
            <button
              key={o || "all"}
              className={`chip-button${outcome === o ? " active" : ""}`}
              onClick={() => setOutcome(o)}
            >
              {o === "" ? t("rec.filter.all") : statusLabel(o)}
            </button>
          ))}
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <label className="subtle" style={{ fontSize: 12 }}>
            {t("rec.filter.from")}
          </label>
          <input
            type="date"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            style={{ width: 140 }}
          />
          <label className="subtle" style={{ fontSize: 12 }}>
            {t("rec.filter.to")}
          </label>
          <input
            type="date"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            style={{ width: 140 }}
          />
          <button className="button compact" onClick={applyFilter}>
            {t("rec.filter.apply")}
          </button>
          <button className="button ghost compact" onClick={clearFilter}>
            {t("rec.filter.clear")}
          </button>
        </div>
      </div>

      {loading ? (
        <TableSkeleton cols={8} rows={6} />
      ) : items.length === 0 && total === 0 ? (
        <EmptyState
          icon={Mic}
          title={t("rec.empty")}
          description={t("rec.subtitle")}
        />
      ) : items.length === 0 ? (
        <EmptyState title={t("rec.empty.filter")} />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("rec.col.lead")}</th>
                <th>{t("rec.col.phone")}</th>
                <th>{t("rec.col.campaign")}</th>
                <th>{t("rec.col.outcome")}</th>
                <th>{t("rec.col.duration")}</th>
                <th>{t("rec.col.size")}</th>
                <th>{t("rec.col.created")}</th>
                <th style={{ minWidth: 260 }}>{t("rec.col.player")}</th>
                <th>{t("rec.col.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {items.map((rec) => (
                <tr key={rec.id}>
                  <td className="primary-cell">
                    <Link href={`/portal/calls/${rec.callId}`} style={{ color: "inherit" }}>
                      {rec.leadName || "—"}
                    </Link>
                  </td>
                  <td>
                    <code className="mono">{rec.phone}</code>
                  </td>
                  <td>{rec.campaign || "—"}</td>
                  <td>
                    <span className={statusClass(rec.outcome)}>{statusLabel(rec.outcome)}</span>
                  </td>
                  <td>{formatDurationShort(rec.durationSec)}</td>
                  <td>{formatBytes(rec.sizeBytes)}</td>
                  <td className="subtle" style={{ fontSize: 12, whiteSpace: "nowrap" }}>
                    {new Date(rec.createdAt).toLocaleString()}
                  </td>
                  <td>
                    {rec.url ? (
                      <audio controls preload="none" src={rec.url} style={{ maxWidth: 260, height: 32 }} />
                    ) : (
                      <span className="subtle">—</span>
                    )}
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    {rec.url ? (
                      <a
                        href={rec.url}
                        download={`${rec.leadName || "call"}-${rec.id}.${extFromContentType(rec.contentType)}`}
                        className="button ghost compact"
                        aria-label={t("rec.btn.download")}
                      >
                        <Download aria-hidden="true" />
                      </a>
                    ) : null}
                    <button
                      className="button ghost compact"
                      onClick={() => handleDelete(rec)}
                      aria-label={t("rec.btn.delete")}
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

      {total > 0 ? (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 16 }}>
          <span className="subtle" style={{ fontSize: 12.5 }}>
            {t("rec.page.count", { shown: `${shownFrom}-${shownTo}`, total: String(total) })}
          </span>
          <div style={{ display: "flex", gap: 6 }}>
            <button
              className="button ghost compact"
              disabled={page <= 1 || loading}
              onClick={() => reload(page - 1)}
            >
              {t("rec.page.prev")}
            </button>
            <button
              className="button ghost compact"
              disabled={page >= totalPages || loading}
              onClick={() => reload(page + 1)}
            >
              {t("rec.page.next")}
            </button>
          </div>
        </div>
      ) : null}
    </>
  );
}

function extFromContentType(ct: string): string {
  if (ct.includes("mpeg")) return "mp3";
  if (ct.includes("ogg")) return "ogg";
  if (ct.includes("opus")) return "opus";
  return "wav";
}
