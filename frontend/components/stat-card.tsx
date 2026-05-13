export function StatCard({
  label,
  value,
  hint,
  trend
}: {
  label: string;
  value: string | number;
  hint: string;
  trend?: string;
}) {
  return (
    <section className="panel stat-card">
      <p className="eyebrow">{label}</p>
      <span className="stat-value">{value}</span>
      {trend ? <span className="stat-trend">{trend}</span> : null}
      <p className="subtle">{hint}</p>
    </section>
  );
}
