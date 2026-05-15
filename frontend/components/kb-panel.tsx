"use client";

// Panel de Knowledge Base en settings.
//
// Operaciones:
//   - Listar documentos del tenant con su estado (pending / processing /
//     ready / failed) y número de chunks.
//   - Subir nuevo: solo TXT/MD por ahora; el backend rechaza el resto.
//   - Eliminar: cascade borra los chunks.
//   - Probar búsqueda: pasa una query al endpoint /api/kb/search y
//     muestra los top-K hits con su score.
//
// Polling cada 8s mientras haya documentos en pending/processing —
// queremos ver el estado "ready" llegar en directo, no obligar al
// operador a refrescar.

import { useEffect, useRef, useState } from "react";
import { FileText, Search, Trash2, Upload } from "lucide-react";
import { useConfirm } from "./confirm";
import { useToast } from "./toast";
import { api, ApiError, KBDocument, KBSearchHit } from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";

export function KBPanel() {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const confirm = useConfirm();
  const fileRef = useRef<HTMLInputElement>(null);
  const [docs, setDocs] = useState<KBDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [needsKey, setNeedsKey] = useState(false);

  async function reload() {
    try {
      const list = await api.kbDocuments(tenant);
      setDocs(list);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      // 404 silencia: tenant sin nada todavía.
      if (code !== "http_404") {
        toast.push(t("kb.toast.error", { err: code }), "danger");
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant]);

  // Polling mientras haya algo procesándose. Si todos están ready/failed
  // dejamos de hacer poll para no quemar requests.
  useEffect(() => {
    const inFlight = docs.some((d) => d.status === "pending" || d.status === "processing");
    if (!inFlight) return;
    const id = window.setInterval(reload, 8000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [docs.map((d) => d.id + ":" + d.status).join(",")]);

  async function handleFile(file: File) {
    if (!file) return;
    if (!/^text\//.test(file.type) && !/\.(txt|md)$/i.test(file.name)) {
      toast.push(t("kb.toast.only_text"), "warn");
      return;
    }
    setUploading(true);
    try {
      await api.uploadKBDocument(file, tenant);
      toast.push(t("kb.toast.uploaded"), "success");
      setNeedsKey(false);
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      if (code === "openai_key_required") {
        setNeedsKey(true);
      } else {
        toast.push(t("kb.toast.error", { err: code }), "danger");
      }
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function handleDelete(doc: KBDocument) {
    const ok = await confirm({
      title: t("kb.btn.delete"),
      description: t("kb.toast.delete_confirm", { name: doc.name }),
      variant: "danger",
      confirmLabel: t("kb.btn.delete"),
    });
    if (!ok) return;
    try {
      await api.deleteKBDocument(doc.id, tenant);
      toast.push(t("kb.toast.deleted"), "success");
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("kb.toast.error", { err: code }), "danger");
    }
  }

  function statusBadge(d: KBDocument) {
    const className =
      d.status === "ready" ? "status good" : d.status === "failed" ? "status danger" : "status warn";
    return <span className={className}>{t(`kb.status.${d.status}`)}</span>;
  }

  return (
    <>
      <section className="panel" style={{ marginTop: 16 }}>
        <div className="panel-header">
          <div>
            <p className="eyebrow">{t("kb.eyebrow")}</p>
            <h2>{t("kb.title")}</h2>
            <p className="subtle" style={{ marginTop: 4 }}>{t("kb.desc")}</p>
          </div>
          <button
            type="button"
            className="button compact"
            onClick={() => fileRef.current?.click()}
            disabled={uploading}
          >
            <Upload aria-hidden="true" />
            <span>{uploading ? t("kb.btn.uploading") : t("kb.btn.upload")}</span>
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".txt,.md,text/plain,text/markdown"
            style={{ display: "none" }}
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) handleFile(f);
            }}
          />
        </div>

        {needsKey ? (
          <div className="form-error" style={{ marginTop: 12 }}>
            {t("kb.needkey")}
          </div>
        ) : null}

        {loading ? (
          <p className="subtle">{t("g.loading")}</p>
        ) : docs.length === 0 ? (
          <p className="subtle">{t("kb.empty")}</p>
        ) : (
          <div className="table-wrap" style={{ marginTop: 12 }}>
            <table>
              <thead>
                <tr>
                  <th>{t("kb.col.name")}</th>
                  <th>{t("kb.col.size")}</th>
                  <th>{t("kb.col.status")}</th>
                  <th>{t("kb.col.chunks")}</th>
                  <th>{t("kb.col.created")}</th>
                  <th>{t("kb.col.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {docs.map((d) => (
                  <tr key={d.id}>
                    <td className="primary-cell">
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                        <FileText aria-hidden="true" style={{ width: 14, height: 14 }} />
                        {d.name}
                      </span>
                      {d.error ? (
                        <p className="subtle" style={{ fontSize: 11.5, marginTop: 2, color: "var(--danger)" }}>
                          {d.error}
                        </p>
                      ) : null}
                    </td>
                    <td>{formatBytes(d.sizeBytes)}</td>
                    <td>{statusBadge(d)}</td>
                    <td>{d.chunkCount}</td>
                    <td className="subtle" style={{ fontSize: 12 }}>
                      {new Date(d.createdAt).toLocaleString()}
                    </td>
                    <td>
                      <button
                        type="button"
                        className="button ghost compact"
                        onClick={() => handleDelete(d)}
                        aria-label={t("kb.btn.delete")}
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
      </section>

      <KBSearchBox />
    </>
  );
}

function KBSearchBox() {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<KBSearchHit[] | null>(null);
  const [searching, setSearching] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!q.trim()) return;
    setSearching(true);
    setHits(null);
    try {
      const r = await api.kbSearch(q.trim(), tenant);
      setHits(r);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("kb.toast.error", { err: code }), "danger");
    } finally {
      setSearching(false);
    }
  }

  return (
    <section className="panel" style={{ marginTop: 16 }}>
      <div className="panel-header">
        <div>
          <p className="eyebrow">{t("kb.eyebrow")}</p>
          <h2>{t("kb.search.title")}</h2>
          <p className="subtle" style={{ marginTop: 4 }}>{t("kb.search.desc")}</p>
        </div>
      </div>
      <form onSubmit={submit} style={{ display: "flex", gap: 8, marginTop: 12 }}>
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={t("kb.search.placeholder")}
          style={{ flex: 1 }}
        />
        <button type="submit" className="button" disabled={searching || !q.trim()}>
          <Search aria-hidden="true" />
          <span>{searching ? t("kb.search.searching") : t("kb.search.button")}</span>
        </button>
      </form>
      {hits !== null ? (
        hits.length === 0 ? (
          <p className="subtle" style={{ marginTop: 12 }}>{t("kb.search.empty")}</p>
        ) : (
          <div style={{ display: "grid", gap: 10, marginTop: 14 }}>
            {hits.map((h, i) => (
              <div key={i} className="command-row" style={{ alignItems: "flex-start" }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                    <span className="chip">{t("kb.search.score", { n: h.score.toFixed(2) })}</span>
                    <span className="subtle" style={{ fontSize: 12 }}>
                      {t("kb.search.fromDoc", { doc: h.document })}
                    </span>
                  </div>
                  <p style={{ marginTop: 6, fontSize: 13, lineHeight: 1.5, whiteSpace: "pre-wrap" }}>
                    {h.chunk}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )
      ) : null}
    </section>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
