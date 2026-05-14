"use client";

import { useEffect } from "react";
import Link from "next/link";
import { BrandMark } from "../components/logo";
import { useT } from "../lib/i18n";

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const t = useT();
  useEffect(() => {
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
          {t("error.tagline.before")} <span className="accent">{t("error.tagline.after")}</span>
        </h1>
        <p className="subtle">{t("error.description")}</p>
      </div>
      <div className="login-form-wrap">
        <div className="login-form">
          <p className="eyebrow">{t("error.eyebrow")}</p>
          <h2>{error.message || t("error.fallback")}</h2>
          {error.digest ? (
            <p className="subtle" style={{ marginBottom: 16 }}>
              {t("error.code.label")} <code className="mono">{error.digest}</code>
            </p>
          ) : null}
          <div className="actions" style={{ gap: 8 }}>
            <button className="button" onClick={() => reset()}>
              {t("error.retry")}
            </button>
            <Link className="button secondary" href="/portal">
              {t("error.back")}
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
