"use client";

// Command palette — abrir con ⌘K / Ctrl+K. Permite:
//   - Buscar leads (nombre / teléfono / email)
//   - Buscar calls (lead / id)
//   - Buscar campañas y bots
//   - Saltar a páginas del menú ("ir a Configuración")
//
// Datos: cacheamos las listas in-memory durante 60s. La búsqueda es
// case-insensitive sobre los campos relevantes. No usamos fuzzy match
// porque para los 50-200 items típicos de un tenant basta con
// substring + ordenación por relevancia simple.
//
// El AppShell monta este componente una sola vez con un listener global
// de teclado. Cmd+K (Mac) y Ctrl+K (Win/Linux) abren el palette desde
// cualquier página.

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Bot, Megaphone, PhoneCall, Search, Settings, Users } from "lucide-react";
import { api, Bot as BotT, Call, Campaign, Lead } from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";

type Result = {
  id: string;
  kind: "lead" | "call" | "campaign" | "bot" | "nav";
  label: string;
  hint?: string;
  href: string;
};

// Páginas estáticas accesibles desde el palette ("ir a …").
const NAV_TARGETS: { labelKey: string; href: string }[] = [
  { labelKey: "nav.dashboard", href: "/portal" },
  { labelKey: "nav.calls", href: "/portal/calls" },
  { labelKey: "nav.leads", href: "/portal/leads" },
  { labelKey: "nav.campaigns", href: "/portal/campaigns" },
  { labelKey: "nav.bots", href: "/portal/bots" },
  { labelKey: "nav.tools", href: "/portal/tools" },
  { labelKey: "nav.properties", href: "/portal/properties" },
  { labelKey: "nav.recordings", href: "/portal/recordings" },
  { labelKey: "nav.billing", href: "/portal/billing" },
  { labelKey: "nav.dnc", href: "/portal/do-not-call" },
  { labelKey: "nav.audit", href: "/portal/audit" },
  { labelKey: "nav.settings", href: "/portal/settings" },
];

// Cache TTL — el operador típico abre el palette repetidas veces en
// segundos, no necesitamos fetch cada vez. 60s es un compromiso entre
// frescura y eficiencia.
const CACHE_TTL = 60_000;

type Cache = {
  ts: number;
  leads: Lead[];
  calls: Call[];
  campaigns: Campaign[];
  bots: BotT[];
};

export function CommandPalette() {
  const router = useRouter();
  const tenant = useTenantScope();
  const t = useT();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const [cache, setCache] = useState<Cache | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Listener global de teclado para ⌘K / Ctrl+K. Escape cierra.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const isMod = e.metaKey || e.ctrlKey;
      if (isMod && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((v) => !v);
        return;
      }
      if (e.key === "Escape" && open) {
        setOpen(false);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  // Cargar cache cuando se abre el palette si está vacía o expirada.
  useEffect(() => {
    if (!open) return;
    const fresh = cache && Date.now() - cache.ts < CACHE_TTL;
    if (fresh) return;
    let cancelled = false;
    (async () => {
      try {
        const [leads, calls, campaigns, bots] = await Promise.all([
          api.leads(tenant).catch(() => [] as Lead[]),
          api.calls(tenant).catch(() => [] as Call[]),
          api.campaigns(tenant).catch(() => [] as Campaign[]),
          api.bots(tenant).catch(() => [] as BotT[]),
        ]);
        if (cancelled) return;
        setCache({ ts: Date.now(), leads, calls, campaigns, bots });
      } catch {
        /* swallow — palette debe seguir usable con navegación */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, tenant, cache]);

  // Reset al abrir.
  useEffect(() => {
    if (open) {
      setQuery("");
      setActive(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const results = useMemo<Result[]>(() => {
    const q = query.trim().toLowerCase();
    const out: Result[] = [];

    // Sin query: mostrar accesos rápidos (navegación) — siempre.
    if (!q) {
      for (const n of NAV_TARGETS) {
        out.push({
          id: `nav:${n.href}`,
          kind: "nav",
          label: t(n.labelKey),
          href: n.href,
        });
      }
      return out;
    }

    if (cache) {
      // Leads — por nombre, phone o email.
      for (const l of cache.leads) {
        const match =
          l.name.toLowerCase().includes(q) ||
          l.phone.includes(q) ||
          (l.email && l.email.toLowerCase().includes(q));
        if (match) {
          out.push({
            id: `lead:${l.id}`,
            kind: "lead",
            label: l.name || l.phone,
            hint: `${l.phone}${l.email ? ` · ${l.email}` : ""}`,
            href: `/portal/leads/${l.id}`,
          });
        }
        if (out.length >= 40) break;
      }
      // Calls — por leadName, phone o id.
      for (const c of cache.calls) {
        const match =
          (c.leadName && c.leadName.toLowerCase().includes(q)) ||
          c.phone.includes(q) ||
          c.id.toLowerCase().includes(q);
        if (match) {
          out.push({
            id: `call:${c.id}`,
            kind: "call",
            label: c.leadName || c.phone,
            hint: `${c.phone} · ${c.status}${c.campaign ? ` · ${c.campaign}` : ""}`,
            href: `/portal/calls/${c.id}`,
          });
        }
        if (out.length >= 80) break;
      }
      // Campañas y bots.
      for (const camp of cache.campaigns) {
        if (camp.name.toLowerCase().includes(q)) {
          out.push({
            id: `camp:${camp.id}`,
            kind: "campaign",
            label: camp.name,
            hint: camp.status,
            href: `/portal/campaigns`,
          });
        }
      }
      for (const b of cache.bots) {
        if (b.name.toLowerCase().includes(q)) {
          out.push({
            id: `bot:${b.id}`,
            kind: "bot",
            label: b.name,
            hint: `${b.language} · ${b.voiceProvider}`,
            href: `/portal/bots`,
          });
        }
      }
    }

    // Navegación que matchea el query (al final, después de datos).
    for (const n of NAV_TARGETS) {
      const label = t(n.labelKey);
      if (label.toLowerCase().includes(q)) {
        out.push({
          id: `nav:${n.href}`,
          kind: "nav",
          label,
          href: n.href,
        });
      }
    }

    return out;
  }, [query, cache, t]);

  // Mantener el índice activo dentro de rango cuando cambia la lista.
  useEffect(() => {
    if (active >= results.length) setActive(Math.max(0, results.length - 1));
  }, [results.length, active]);

  function pick(r: Result) {
    setOpen(false);
    router.push(r.href);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => Math.min(results.length - 1, a + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(0, a - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const r = results[active];
      if (r) pick(r);
    }
  }

  if (!open) return null;

  return (
    <div className="cmdk-overlay" role="dialog" aria-modal="true" onKeyDown={onKeyDown}>
      <div className="cmdk-backdrop" onClick={() => setOpen(false)} />
      <div className="cmdk-panel">
        <div className="cmdk-input-row">
          <Search size={18} aria-hidden="true" />
          <input
            ref={inputRef}
            className="cmdk-input"
            placeholder={t("cmdk.placeholder")}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            autoFocus
          />
          <kbd className="cmdk-kbd">ESC</kbd>
        </div>
        <div className="cmdk-results">
          {results.length === 0 ? (
            <div className="cmdk-empty">{t("cmdk.empty")}</div>
          ) : (
            results.map((r, i) => (
              <button
                key={r.id}
                type="button"
                className={`cmdk-item${i === active ? " active" : ""}`}
                onMouseEnter={() => setActive(i)}
                onClick={() => pick(r)}
              >
                <span className="cmdk-icon">{iconFor(r.kind)}</span>
                <div className="cmdk-text">
                  <span className="cmdk-label">{r.label}</span>
                  {r.hint ? <span className="cmdk-hint">{r.hint}</span> : null}
                </div>
                <span className="cmdk-kind">{t(`cmdk.kind.${r.kind}`)}</span>
              </button>
            ))
          )}
        </div>
        <div className="cmdk-footer">
          <span>
            <kbd className="cmdk-kbd">↑↓</kbd> {t("cmdk.foot.nav")}
          </span>
          <span>
            <kbd className="cmdk-kbd">↵</kbd> {t("cmdk.foot.open")}
          </span>
          <span style={{ marginLeft: "auto" }}>
            <kbd className="cmdk-kbd">⌘K</kbd>
          </span>
        </div>
      </div>
    </div>
  );
}

function iconFor(kind: Result["kind"]) {
  switch (kind) {
    case "lead":
      return <Users size={16} />;
    case "call":
      return <PhoneCall size={16} />;
    case "campaign":
      return <Megaphone size={16} />;
    case "bot":
      return <Bot size={16} />;
    default:
      return <Settings size={16} />;
  }
}
