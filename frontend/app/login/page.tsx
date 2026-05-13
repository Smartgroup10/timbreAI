"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError } from "../../lib/api";
import { useAuth } from "../../lib/auth-context";

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
        setError("No se pudo conectar con el servidor");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="login-shell">
      <div className="login-art">
        <div className="login-brand">
          <div className="brand-mark">CH</div>
          <div>
            <strong>CallHub</strong>
            <span>Voice AI operations</span>
          </div>
        </div>
        <h1>El centro de mando de tus llamadas IA.</h1>
        <p className="subtle">
          Configura bots, ejecuta campanas, supervisa transferencias y compliance desde un solo panel.
          Multi-tenant, multi-canal, multi-bot.
        </p>
        <ul className="login-feature-list">
          <li>Postgres + multi-tenant aislado por JWT.</li>
          <li>ARI de Asterisk para origen de llamadas reales.</li>
          <li>Trunk SIP configurable (Twilio, Vonage, Telnyx).</li>
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
            />
          </div>
          <div className="field">
            <label htmlFor="password">Contrasena</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>

          {error ? <div className="form-error">{error}</div> : null}

          <button className="button" disabled={submitting}>
            {submitting ? "Entrando…" : "Entrar"}
          </button>

          <p className="login-hint subtle">
            Credenciales de demo: <code>owner@atrium.local</code> / <code>atrium123</code>
            <br />
            Admin de plataforma: <code>admin@callhub.local</code> / <code>atrium123</code>
          </p>
        </form>
      </div>
    </div>
  );
}

function translateError(code: string): string {
  switch (code) {
    case "invalid_credentials":
      return "Email o contrasena incorrectos.";
    case "email_and_password_required":
      return "Introduce email y contrasena.";
    default:
      return code;
  }
}
