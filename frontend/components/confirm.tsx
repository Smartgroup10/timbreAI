"use client";

// Modal de confirmación accesible. Reemplaza window.confirm() por un dialog
// real (foco al botón principal, Esc/Enter, ARIA), preservando la API
// síncrona-ish con una promesa:
//
//   const confirm = useConfirm();
//   const ok = await confirm({ title, description, variant: "danger" });
//   if (!ok) return;
//
// El provider mantiene una sola petición a la vez — abrir otra mientras hay
// una abierta resuelve la anterior como false y muestra la nueva.

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useT } from "../lib/i18n";

type Variant = "default" | "danger";

type Options = {
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: Variant;
};

type Pending = Options & {
  resolve: (ok: boolean) => void;
};

type Ctx = (opts: Options) => Promise<boolean>;

const ConfirmContext = createContext<Ctx | null>(null);

export function ConfirmProvider({ children }: { children: React.ReactNode }) {
  const [pending, setPending] = useState<Pending | null>(null);
  const confirmBtnRef = useRef<HTMLButtonElement | null>(null);

  const ask = useCallback<Ctx>((opts) => {
    return new Promise<boolean>((resolve) => {
      setPending((prev) => {
        // Si había uno abierto, lo cancelamos antes de mostrar el nuevo.
        if (prev) prev.resolve(false);
        return { ...opts, resolve };
      });
    });
  }, []);

  function resolve(ok: boolean) {
    if (!pending) return;
    pending.resolve(ok);
    setPending(null);
  }

  // Foco al botón de confirmación cuando se abre — para que Enter funcione.
  useEffect(() => {
    if (!pending) return;
    const id = window.setTimeout(() => {
      confirmBtnRef.current?.focus();
    }, 0);
    return () => window.clearTimeout(id);
  }, [pending]);

  // Esc cierra (=cancel). Enter no se intercepta aquí; al estar el botón
  // confirmar enfocado, Enter ya lo activa nativamente.
  useEffect(() => {
    if (!pending) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        resolve(false);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pending]);

  const value = useMemo(() => ask, [ask]);

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      {pending ? <ConfirmDialog pending={pending} confirmBtnRef={confirmBtnRef} onResolve={resolve} /> : null}
    </ConfirmContext.Provider>
  );
}

function ConfirmDialog({
  pending,
  confirmBtnRef,
  onResolve,
}: {
  pending: Pending;
  confirmBtnRef: React.MutableRefObject<HTMLButtonElement | null>;
  onResolve: (ok: boolean) => void;
}) {
  const t = useT();
  const variant = pending.variant ?? "default";
  const confirmLabel = pending.confirmLabel ?? t("confirm.confirm");
  const cancelLabel = pending.cancelLabel ?? t("confirm.cancel");

  return (
    <div className="confirm-overlay" role="presentation">
      <button
        type="button"
        className="confirm-backdrop"
        aria-label={cancelLabel}
        onClick={() => onResolve(false)}
      />
      <div
        className={`confirm-dialog confirm-${variant}`}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby={pending.description ? "confirm-desc" : undefined}
      >
        <h2 id="confirm-title" className="confirm-title">
          {pending.title}
        </h2>
        {pending.description ? (
          <p id="confirm-desc" className="confirm-desc">
            {pending.description}
          </p>
        ) : null}
        <div className="confirm-actions">
          <button
            type="button"
            className="button secondary"
            onClick={() => onResolve(false)}
          >
            {cancelLabel}
          </button>
          <button
            ref={confirmBtnRef}
            type="button"
            className={`button${variant === "danger" ? " danger" : ""}`}
            onClick={() => onResolve(true)}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

export function useConfirm(): Ctx {
  const ctx = useContext(ConfirmContext);
  if (!ctx) throw new Error("useConfirm must be used inside ConfirmProvider");
  return ctx;
}
