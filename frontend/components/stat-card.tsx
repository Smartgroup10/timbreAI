export function StatCard({ label, value, hint }: { label: string; value: string | number; hint: string }) {
  return (
    <section className="panel">
      <p className="eyebrow">{label}</p>
      <span className="stat-value">{value}</span>
      <p className="subtle">{hint}</p>
    </section>
  );
}

