import Link from "next/link";
import { BrandMark } from "../components/logo";

// Página 404 personalizada. Next.js la sirve automáticamente cuando una ruta
// no machea — el default es feo y técnico, esta es coherente con la marca
// y empuja al usuario al portal con un solo click.
export default function NotFound() {
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
          Aquí no hay <span className="accent">nada que sonar.</span>
        </h1>
        <p className="subtle">
          La página que buscas no existe o fue movida. Vuelve al portal o al login para seguir
          operando.
        </p>
      </div>
      <div className="login-form-wrap">
        <div className="login-form">
          <p className="eyebrow">Error 404</p>
          <h2>Página no encontrada</h2>
          <p className="subtle">
            Comprueba la URL o usa el menú lateral del portal para navegar.
          </p>
          <Link className="button" href="/portal" style={{ marginTop: 16, display: "inline-block" }}>
            Ir al portal
          </Link>
        </div>
      </div>
    </div>
  );
}
