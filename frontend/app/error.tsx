"use client";

// Error boundary global de Next.js App Router. Next la invoca cuando un
// componente del árbol lanza durante render — antes el usuario veía un
// white screen sin contexto.
import { useEffect } from "react";
import Link from "next/link";
import { BrandMark } from "../components/logo";

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    // Log al console del navegador — el operador con DevTools abierto verá
    // el stack y la digest para abrirnos un ticket.
    console.error("timbre.ai · global error boundary:", error);
  }, [error]);

  return (
    <div className="login-shell">
      <div className="login-art">
        <div className="login-brand-row">
          <BrandMark size={44} />
          <div className="login-brand-name">
            timbre<span>.ai</span>
          </div>
        </div>
        <h1>
          Algo salió <span className="accent">mal.</span>
        </h1>
        <p className="subtle">
          Tu sesión sigue activa. Reintenta la acción; si vuelve a fallar, contacta con soporte
          con el código de abajo.
        </p>
      </div>
      <div className="login-form-wrap">
        <div className="login-form">
          <p className="eyebrow">Error inesperado</p>
          <h2>{error.message || "Fallo al renderizar la página"}</h2>
          {error.digest ? (
            <p className="subtle" style={{ marginBottom: 16 }}>
              Código: <code className="mono">{error.digest}</code>
            </p>
          ) : null}
          <div className="actions" style={{ gap: 8 }}>
            <button className="button" onClick={() => reset()}>
              Reintentar
            </button>
            <Link className="button secondary" href="/portal">
              Volver al portal
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
