"use client";

// Density preference para tablas — "comfortable" (default) o "compact".
// Persistimos en localStorage para que el usuario no tenga que volver
// a marcarlo cada sesión. El AppShell propaga el estado al <html> via
// `data-density="compact"` para que CSS global reduzca paddings.

import { useEffect, useState } from "react";

const KEY = "timbre.tableDensity";
export type Density = "comfortable" | "compact";

export function useDensity(): [Density, (d: Density) => void] {
  const [density, setDensityState] = useState<Density>("comfortable");

  // Leer del localStorage al montar. En SSR no existe `window` así que
  // hacemos esto en effect para evitar hydration mismatch.
  useEffect(() => {
    try {
      const v = window.localStorage.getItem(KEY);
      if (v === "compact" || v === "comfortable") {
        setDensityState(v);
      }
    } catch {
      /* private mode / disabled storage — ignore */
    }
  }, []);

  // Aplicar al <html> y persistir.
  useEffect(() => {
    if (typeof document === "undefined") return;
    document.documentElement.dataset.density = density;
    try {
      window.localStorage.setItem(KEY, density);
    } catch {
      /* ignore */
    }
  }, [density]);

  return [density, setDensityState];
}
