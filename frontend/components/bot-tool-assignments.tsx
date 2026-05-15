"use client";

// BotToolAssignments — sección dentro del editor del bot.
//
// Antes: el operador creaba/editaba/borraba tools aquí.
// Ahora: solo SELECCIONA cuáles de la biblioteca asignar al bot, con un
// switch enabled por cada una. Para crear/editar tools se va a la
// página /portal/tools (link explícito al final).
//
// Cada cambio de switch hace un PUT/DELETE inmediato — sin botón
// "guardar". Optimista: actualizamos local state al toque y revertimos
// si la petición falla.

import { useEffect, useState } from "react";
import Link from "next/link";
import { ExternalLink } from "lucide-react";
import { useToast } from "./toast";
import { api, ApiError, BotToolView } from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";

export function BotToolAssignments({ botId }: { botId: string }) {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const [views, setViews] = useState<BotToolView[]>([]);
  const [loading, setLoading] = useState(true);

  async function reload() {
    setLoading(true);
    try {
      const list = await api.botToolAssignments(botId, tenant);
      setViews(list);
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
  }, [botId, tenant]);

  // Asignar = PUT { enabled:true }. Desasignar = DELETE.
  // Toggle enabled de una ya asignada = PUT { enabled:!current }.
  async function handleAssignToggle(view: BotToolView, nextAssigned: boolean) {
    // Optimismo local — invertimos antes de la red.
    setViews((prev) =>
      prev.map((v) =>
        v.id === view.id
          ? { ...v, assigned: nextAssigned, assignedEnabled: nextAssigned ? true : false }
          : v
      )
    );
    try {
      if (nextAssigned) {
        await api.assignToolToBot(botId, view.id, true, tenant);
      } else {
        await api.unassignToolFromBot(botId, view.id, tenant);
      }
    } catch (err) {
      // Revertir.
      setViews((prev) => prev.map((v) => (v.id === view.id ? view : v)));
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  async function handleEnabledToggle(view: BotToolView, nextEnabled: boolean) {
    setViews((prev) =>
      prev.map((v) => (v.id === view.id ? { ...v, assignedEnabled: nextEnabled } : v))
    );
    try {
      await api.assignToolToBot(botId, view.id, nextEnabled, tenant);
    } catch (err) {
      setViews((prev) => prev.map((v) => (v.id === view.id ? view : v)));
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("tools.toast.error", { err: code }), "danger");
    }
  }

  return (
    <section style={{ marginTop: 20, paddingTop: 20, borderTop: "1px solid var(--border)" }}>
      <div className="panel-header" style={{ marginBottom: 8 }}>
        <div>
          <p className="eyebrow">{t("tools.assign.eyebrow")}</p>
          <h3 style={{ margin: 0, fontSize: 15 }}>{t("tools.assign.title")}</h3>
          <p className="subtle" style={{ marginTop: 4, fontSize: 12.5 }}>
            {t("tools.assign.desc")}{" "}
            <Link href="/portal/tools" style={{ color: "var(--coral)" }}>
              {t("tools.assign.gotolibrary")} <ExternalLink aria-hidden="true" style={{ width: 11, height: 11, verticalAlign: "middle" }} />
            </Link>
          </p>
        </div>
      </div>

      {loading ? (
        <p className="subtle">{t("g.loading")}</p>
      ) : views.length === 0 ? (
        <p className="subtle" style={{ fontSize: 12.5 }}>{t("tools.assign.library.empty")}</p>
      ) : (
        <div style={{ display: "grid", gap: 8 }}>
          {views.map((v) => (
            <div
              key={v.id}
              className="command-row"
              style={{ alignItems: "flex-start", padding: "10px 12px" }}
            >
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  <strong className="mono" style={{ fontSize: 13 }}>{v.name}</strong>
                  <span className="chip">{t(`tools.action.${v.actionType}`)}</span>
                  {!v.enabled ? (
                    <span className="status warn">{t("tools.assign.archived")}</span>
                  ) : null}
                </div>
                <p className="subtle" style={{ marginTop: 4, fontSize: 12.5 }}>{v.description}</p>
              </div>
              <div style={{ display: "flex", flexDirection: "column", gap: 4, flexShrink: 0, alignItems: "flex-end" }}>
                <label
                  className="checkbox-row"
                  title={t("tools.assign.assigned.hint")}
                  style={{ fontSize: 11.5 }}
                >
                  <input
                    type="checkbox"
                    checked={v.assigned}
                    onChange={(e) => handleAssignToggle(v, e.target.checked)}
                    disabled={!v.enabled && !v.assigned /* tools archivadas solo se pueden quitar */}
                  />
                  <span>{t("tools.assign.assigned")}</span>
                </label>
                {v.assigned ? (
                  <label
                    className="checkbox-row"
                    title={t("tools.assign.enabled.hint")}
                    style={{ fontSize: 11.5 }}
                  >
                    <input
                      type="checkbox"
                      checked={v.assignedEnabled}
                      onChange={(e) => handleEnabledToggle(v, e.target.checked)}
                    />
                    <span>{t("tools.assign.enabled")}</span>
                  </label>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
