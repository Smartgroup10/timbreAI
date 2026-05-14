"use client";

import Link from "next/link";
import { BrandMark } from "../components/logo";
import { useT } from "../lib/i18n";

// Página 404 personalizada. Next.js la sirve automáticamente cuando una ruta
// no machea — el default es feo y técnico, esta es coherente con la marca
// y empuja al usuario al portal con un solo click.
export default function NotFound() {
  const t = useT();
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
          {t("notfound.tagline.before")} <span className="accent">{t("notfound.tagline.after")}</span>
        </h1>
        <p className="subtle">{t("notfound.description")}</p>
      </div>
      <div className="login-form-wrap">
        <div className="login-form">
          <p className="eyebrow">{t("notfound.eyebrow")}</p>
          <h2>{t("notfound.title")}</h2>
          <p className="subtle">{t("notfound.body")}</p>
          <Link className="button" href="/portal" style={{ marginTop: 16, display: "inline-block" }}>
            {t("notfound.back")}
          </Link>
        </div>
      </div>
    </div>
  );
}
