"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "./api";

type State<T> = {
  data: T | null;
  loading: boolean;     // true solo en la primera carga
  refreshing: boolean;  // true en re-fetches background (polling/reload)
  error: string | null;
};

type Options = {
  // Si > 0, refresca cada N ms en background sin marcar loading. El polling
  // se pausa cuando la pestaña no está visible para no quemar CPU/cuota.
  pollMs?: number;
};

export function useResource<T>(
  loader: () => Promise<T>,
  deps: unknown[] = [],
  options: Options = {},
): State<T> & { reload: () => void } {
  const [state, setState] = useState<State<T>>({ data: null, loading: true, refreshing: false, error: null });
  // hasData refleja si ya tenemos data — usado para decidir entre loading
  // (primera carga) y refreshing (refetch). Vive en ref para que reload
  // sea estable y no cause un loop con el polling effect.
  const hasData = useRef(false);

  const reload = useCallback(() => {
    setState((s) => {
      if (hasData.current) return { ...s, refreshing: true, error: null };
      return { ...s, loading: true, error: null };
    });
    loader()
      .then((data) => {
        hasData.current = true;
        setState({ data, loading: false, refreshing: false, error: null });
      })
      .catch((err) => {
        const message = err instanceof ApiError ? err.code : err instanceof Error ? err.message : "error";
        setState((s) => ({ data: s.data, loading: false, refreshing: false, error: message }));
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  // Carga inicial + re-carga al cambiar deps (mediante reload).
  useEffect(() => {
    hasData.current = false;
    reload();
  }, [reload]);

  // Polling background. Pausa cuando la pestaña no está visible.
  useEffect(() => {
    const interval = options.pollMs;
    if (!interval || interval <= 0) return;

    let timerId: number | null = null;
    function schedule() {
      if (timerId !== null) window.clearInterval(timerId);
      timerId = window.setInterval(() => {
        if (document.visibilityState !== "visible") return;
        reload();
      }, interval);
    }
    function onVisibility() {
      if (document.visibilityState === "visible") {
        reload();
        schedule();
      } else if (timerId !== null) {
        window.clearInterval(timerId);
        timerId = null;
      }
    }
    schedule();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      if (timerId !== null) window.clearInterval(timerId);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [reload, options.pollMs]);

  return { ...state, reload };
}
