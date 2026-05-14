"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "../../lib/api";
import { useAuth } from "../../lib/auth-context";
import { useT, useLang } from "../../lib/i18n";
import { BrandMark } from "../../components/logo";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
  const t = useT();
  const { lang, setLang } = useLang();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      const user = await login(email.trim(), password);
      router.replace(user.role === "platform_admin" ? "/admin" : "/portal");
    } catch (err) {
      if (err instanceof ApiError) {
        const key = `login.err.${err.code}`;
        const translated = t(key);
        setError(translated === key ? t("login.err.unexpected", { code: err.code }) : translated);
      } else {
        setError(t("login.err.network"));
      }
    } finally {
      setSubmitting(false);
    }
  }

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
          {t("login.tagline.before")} <span className="accent">{t("login.tagline.after")}</span>
        </h1>
        <p className="subtle">{t("login.description")}</p>
        <ul className="login-feature-list">
          <li>{t("login.feature.1")}</li>
          <li>{t("login.feature.2")}</li>
          <li>{t("login.feature.3")}</li>
        </ul>
      </div>
      <div className="login-form-wrap">
        <form className="login-form" onSubmit={handleSubmit}>
          <div className="login-lang-row">
            <button
              type="button"
              className={`lang-switch-btn${lang === "es" ? " active" : ""}`}
              onClick={() => setLang("es")}
              aria-pressed={lang === "es"}
            >
              ES
            </button>
            <button
              type="button"
              className={`lang-switch-btn${lang === "en" ? " active" : ""}`}
              onClick={() => setLang("en")}
              aria-pressed={lang === "en"}
            >
              EN
            </button>
          </div>
          <p className="eyebrow">{t("login.eyebrow")}</p>
          <h2>{t("login.title")}</h2>
          <p className="subtle">{t("login.subtitle")}</p>

          <div className="field">
            <label htmlFor="email">{t("login.email")}</label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
              placeholder="tu@empresa.com"
            />
          </div>
          <div className="field">
            <label htmlFor="password">{t("login.password")}</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
            />
          </div>

          {error ? (
            <div className="form-error" role="alert">
              {error}
            </div>
          ) : null}

          <button className="button" disabled={submitting}>
            {submitting ? t("login.button.loading") : t("login.button")}
          </button>

          <p className="login-hint subtle">{t("login.forgot")}</p>
        </form>
      </div>
    </div>
  );
}
