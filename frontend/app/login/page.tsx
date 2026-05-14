"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "../../lib/api";
import { useAuth } from "../../lib/auth-context";
import { BrandMark } from "../../components/logo";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
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
        setError(translateError(err.code));
      } else {
        setError("No pudimos conectar con el servidor. Reintenta en unos segundos.");
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
          Cada negocio merece <span className="accent">su propio timbre.</span>
        </h1>
        <p className="subtle">
          Agentes de voz IA que llaman, agendan y responden con el tono de cada marca.
          Configúralos, mide cada llamada, transfiere a humano cuando toque.
        </p>
        <ul className="login-feature-list">
          <li>Multi-tenant aislado, audit log y DNC desde el día uno.</li>
          <li>Voz en tiempo real: OpenAI Realtime, Deepgram, AssemblyAI.</li>
          <li>Asterisk + cualquier trunk SIP (Twilio, Vonage, Telnyx).</li>
        </ul>
      </div>
      <div className="login-form-wrap">
        <form className="login-form" onSubmit={handleSubmit}>
          <p className="eyebrow">Acceso</p>
          <h2>Entrar al portal</h2>
          <p className="subtle">Usa tu cuenta de operador o platform admin.</p>

          <div className="field">
            <label htmlFor="email">Email</label>
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
            <label htmlFor="password">Contraseña</label>
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
            {submitting ? "Entrando…" : "Entrar"}
          </button>

          <p className="login-hint subtle">
            ¿Has olvidado tu contraseña? Habla con el administrador de tu cuenta para que te
            restablezca el acceso.
          </p>
        </form>
      </div>
    </div>
  );
}

function translateError(code: string): string {
  switch (code) {
    case "invalid_credentials":
      return "Email o contraseña incorrectos.";
    case "email_and_password_required":
      return "Introduce email y contraseña.";
    case "tenant_required":
      return "Tu cuenta no tiene tenant asignado. Contacta con el administrador.";
    default:
      return `Error inesperado (${code}). Reintenta o contacta con soporte.`;
  }
}
