"use client";

// Sección Google Calendar dentro del editor de bot.
//
// Flow:
//  1. Pedir authUrl al backend (POST /api/bots/:id/calendar/authorize).
//  2. Abrir esa URL en una popup. Google muestra consent screen.
//  3. Google redirige a /api/calendar/google/callback con ?code.
//  4. Backend valida state firmado, intercambia tokens, guarda integration.
//  5. Página de callback hace window.opener.postMessage({type:"timbre.calendar.connected"}).
//  6. Este componente escucha y refresca el estado.
//
// Si no hay window.opener, el operador refresca a mano — el estado se
// vuelve a leer al pulsar el botón.

import { useEffect, useState } from "react";
import { Calendar, Link2Off } from "lucide-react";
import { useConfirm } from "./confirm";
import { useToast } from "./toast";
import { api, ApiError } from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";

type Status = {
  connected: boolean;
  accountEmail?: string;
};

export function BotCalendarSection({ botId }: { botId: string }) {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const confirm = useConfirm();
  const [status, setStatus] = useState<Status | null>(null);
  const [connecting, setConnecting] = useState(false);
  const [notConfigured, setNotConfigured] = useState(false);

  async function reload() {
    try {
      const r = await api.calendarStatus(botId, tenant);
      setStatus(r);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      if (code === "calendar_not_configured") {
        setNotConfigured(true);
      } else {
        toast.push(t("cal.toast.error", { err: code }), "danger");
      }
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [botId, tenant]);

  // El callback HTML del backend hace postMessage cuando termina. Si
  // el usuario lo cierra antes, no llega nada y se queda sin refrescar
  // — recargar la sección o re-abrir el editor lo arregla.
  useEffect(() => {
    function onMsg(e: MessageEvent) {
      if (e.data?.type === "timbre.calendar.connected") {
        if (e.data.ok) {
          toast.push(t("cal.toast.connected"), "success");
        }
        reload();
      }
    }
    window.addEventListener("message", onMsg);
    return () => window.removeEventListener("message", onMsg);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleConnect() {
    setConnecting(true);
    try {
      const r = await api.calendarAuthorize(botId, tenant);
      // Popup centrada para que el consent screen de Google no abra una
      // pestaña perdida. Tamaño cómodo.
      const w = 520;
      const h = 640;
      const left = window.screenX + (window.outerWidth - w) / 2;
      const top = window.screenY + (window.outerHeight - h) / 2;
      window.open(
        r.authUrl,
        "timbre_calendar_oauth",
        `width=${w},height=${h},left=${left},top=${top}`
      );
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      if (code === "calendar_not_configured") {
        setNotConfigured(true);
      } else {
        toast.push(t("cal.toast.error", { err: code }), "danger");
      }
    } finally {
      setConnecting(false);
    }
  }

  async function handleDisconnect() {
    const ok = await confirm({
      title: t("cal.btn.disconnect"),
      description: t("cal.btn.disconnect.confirm"),
      variant: "danger",
      confirmLabel: t("cal.btn.disconnect"),
    });
    if (!ok) return;
    try {
      await api.calendarDisconnect(botId, tenant);
      toast.push(t("cal.toast.disconnected"), "success");
      await reload();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("cal.toast.error", { err: code }), "danger");
    }
  }

  return (
    <section style={{ marginTop: 20, paddingTop: 20, borderTop: "1px solid var(--border)" }}>
      <div className="panel-header" style={{ marginBottom: 8 }}>
        <div>
          <p className="eyebrow">{t("cal.eyebrow")}</p>
          <h3 style={{ margin: 0, fontSize: 15 }}>{t("cal.title")}</h3>
          <p className="subtle" style={{ marginTop: 4, fontSize: 12.5 }}>{t("cal.desc")}</p>
        </div>
      </div>

      {notConfigured ? (
        <p className="subtle" style={{ fontSize: 12.5 }}>{t("cal.notConfigured")}</p>
      ) : status === null ? (
        <p className="subtle">{t("g.loading")}</p>
      ) : status.connected ? (
        <div className="command-row" style={{ alignItems: "center" }}>
          <div style={{ flex: 1 }}>
            <div style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
              <Calendar aria-hidden="true" style={{ width: 16, height: 16 }} />
              <span className="status good">
                {t("cal.status.connected", { email: status.accountEmail ?? "?" })}
              </span>
            </div>
          </div>
          <button
            type="button"
            className="button ghost compact"
            onClick={handleDisconnect}
            aria-label={t("cal.btn.disconnect")}
          >
            <Link2Off aria-hidden="true" />
            <span>{t("cal.btn.disconnect")}</span>
          </button>
        </div>
      ) : (
        <button
          type="button"
          className="button"
          onClick={handleConnect}
          disabled={connecting}
        >
          <Calendar aria-hidden="true" />
          <span>{connecting ? t("cal.btn.connecting") : t("cal.btn.connect")}</span>
        </button>
      )}
    </section>
  );
}
