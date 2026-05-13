"use client";

// Small zero-dependency SVG charts. Two shapes:
//   - DailyBars: 7-day vertical bar chart with axis-less style.
//   - HBars: horizontal breakdown for outcomes/top lists.
// Both honour the design tokens via currentColor and small color hints.

export function DailyBars({ data, height = 120 }: { data: { date: string; count: number }[]; height?: number }) {
  const max = Math.max(1, ...data.map((d) => d.count));
  const barW = 100 / Math.max(1, data.length);
  return (
    <div className="daily-bars" style={{ height }}>
      <svg viewBox={`0 0 100 ${height}`} preserveAspectRatio="none" style={{ width: "100%", height }}>
        {data.map((d, i) => {
          const h = (d.count / max) * (height - 18);
          const x = i * barW + barW * 0.18;
          const y = height - h - 14;
          const w = barW * 0.64;
          const day = new Date(d.date).toLocaleDateString(undefined, { weekday: "short" }).slice(0, 3);
          return (
            <g key={d.date}>
              <rect x={x} y={y} width={w} height={h || 1} rx={1.5} fill="var(--accent)" opacity={0.85} />
              <text
                x={x + w / 2}
                y={y - 3}
                textAnchor="middle"
                fontSize="6"
                fill="var(--text)"
                fontFamily="var(--font-sans)"
                fontWeight={600}
              >
                {d.count > 0 ? d.count : ""}
              </text>
              <text
                x={x + w / 2}
                y={height - 3}
                textAnchor="middle"
                fontSize="5.5"
                fill="var(--quiet)"
                fontFamily="var(--font-sans)"
              >
                {day}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

const OUTCOME_COLORS: Record<string, string> = {
  qualified: "var(--success)",
  callback: "var(--warning)",
  pending: "var(--quiet)",
  no_answer: "#9ca3af",
  busy: "#6b7280",
  failed: "var(--danger)",
  unreachable: "var(--danger)",
};

export function HBars({ data, accent }: { data: { label: string; count: number }[]; accent?: string }) {
  if (!data || data.length === 0) {
    return <p className="subtle">Sin datos en los últimos 30 días.</p>;
  }
  const max = Math.max(1, ...data.map((d) => d.count));
  return (
    <div className="hbars">
      {data.map((d) => {
        const pct = (d.count / max) * 100;
        const color = accent || OUTCOME_COLORS[d.label] || "var(--accent)";
        return (
          <div className="hbar" key={d.label}>
            <span className="hbar-label">{d.label}</span>
            <div className="hbar-track">
              <span className="hbar-fill" style={{ width: `${pct}%`, background: color }} />
            </div>
            <span className="hbar-value">{d.count}</span>
          </div>
        );
      })}
    </div>
  );
}
