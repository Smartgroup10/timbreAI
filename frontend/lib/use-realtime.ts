"use client";

// Hook de cliente para conectar al endpoint /api/realtime y reaccionar
// a eventos del backend. Reemplaza el polling agresivo de las listas
// (calls, leads, dnc) — el backend hace push cuando hay cambios.
//
// Patrón de uso:
//
//   const { connected } = useRealtime((ev) => {
//     if (ev.type === "call.finished") reloadCalls();
//   });
//
// Reconexión automática con backoff exponencial (1s, 2s, 4s, ..., capped a
// 30s). El componente queda intacto si pierde conexión — el listener no se
// dispara hasta que vuelva.

import { useEffect, useRef, useState } from "react";
import { useAuth, useTenantScope } from "./auth-context";
import { getToken } from "./api";

export type RealtimeEvent = {
  type: string;
  tenantId: string;
  data?: Record<string, unknown>;
};

type Callback = (ev: RealtimeEvent) => void;

export function useRealtime(onEvent: Callback) {
  const { user } = useAuth();
  const tenant = useTenantScope();
  const [connected, setConnected] = useState(false);
  // Guardar callback en ref para que cambios entre renders no reconecten.
  const callbackRef = useRef<Callback>(onEvent);
  callbackRef.current = onEvent;

  useEffect(() => {
    if (!user) return; // sin login no abrimos socket
    const token = getToken();
    if (!token) return;

    let ws: WebSocket | null = null;
    let cancelled = false;
    let attempt = 0;
    let retryTimer: number | null = null;

    function connect() {
      if (cancelled) return;
      // window.location.protocol === "https:" → wss; si no ws.
      const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
      // Endpoint pasa por el mismo origen porque Next dev usa rewrites
      // o el reverse proxy en prod. El backend está bajo /api.
      const base = `${proto}//${window.location.host}/api/realtime?token=${encodeURIComponent(token!)}`;
      const url = tenant ? `${base}&tenant=${encodeURIComponent(tenant)}` : base;
      ws = new WebSocket(url);

      ws.onopen = () => {
        attempt = 0;
        setConnected(true);
      };
      ws.onmessage = (e) => {
        try {
          const ev = JSON.parse(e.data) as RealtimeEvent;
          callbackRef.current(ev);
        } catch {
          // ignore non-json frames
        }
      };
      ws.onerror = () => {
        // onclose lo gestiona; aquí solo loguear sería ruido.
      };
      ws.onclose = () => {
        setConnected(false);
        if (cancelled) return;
        // Backoff exponencial 1s, 2s, 4s, … capped a 30s.
        const delay = Math.min(30_000, 1000 * Math.pow(2, attempt));
        attempt += 1;
        retryTimer = window.setTimeout(connect, delay);
      };
    }

    connect();
    return () => {
      cancelled = true;
      if (retryTimer !== null) window.clearTimeout(retryTimer);
      if (ws) {
        ws.close();
        ws = null;
      }
    };
  }, [user, tenant]);

  return { connected };
}
