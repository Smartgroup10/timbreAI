"use client";

// EmptyState — bloque centrado con título, descripción opcional y CTA.
// Reemplaza los <div className="empty-state">…</div> sueltos cuando hace
// falta empujar al usuario hacia la siguiente acción.

import type { LucideIcon } from "lucide-react";
import Link from "next/link";

type Action =
  | { label: string; href: string; onClick?: never }
  | { label: string; onClick: () => void; href?: never };

type Props = {
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: Action;
  secondary?: Action;
  variant?: "default" | "danger";
};

export function EmptyState({ icon: Icon, title, description, action, secondary, variant = "default" }: Props) {
  return (
    <div className={`empty-state-block${variant === "danger" ? " danger" : ""}`}>
      {Icon ? (
        <div className="empty-state-icon" aria-hidden="true">
          <Icon />
        </div>
      ) : null}
      <h3 className="empty-state-title">{title}</h3>
      {description ? <p className="empty-state-desc subtle">{description}</p> : null}
      {action || secondary ? (
        <div className="empty-state-actions">
          {action ? renderAction(action, "button") : null}
          {secondary ? renderAction(secondary, "button secondary") : null}
        </div>
      ) : null}
    </div>
  );
}

function renderAction(a: Action, className: string) {
  if (a.href) {
    return (
      <Link className={className} href={a.href}>
        {a.label}
      </Link>
    );
  }
  return (
    <button type="button" className={className} onClick={a.onClick}>
      {a.label}
    </button>
  );
}
