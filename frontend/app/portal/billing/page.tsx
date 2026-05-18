"use client";

// /portal/billing — coste real por llamada con breakdown por componente.
//
// El voice-agent reporta usage al cerrar cada sesión; el backend agrega
// por día y por dimensión (provider | campaña). Esta página muestra:
//   - 3 cards con totales (llamadas, minutos, coste)
//   - filtros de rango y groupBy
//   - tabla de filas día × bucket
//
// Las cifras vienen en micro-céntimos para precisión. Formateamos en USD.

import { useEffect, useMemo, useState } from "react";
import { Receipt } from "lucide-react";
import { EmptyState } from "../../../components/empty";
import { TableSkeleton } from "../../../components/skeleton";
import { useToast } from "../../../components/toast";
import {
  api,
  ApiError,
  BillingSummary,
  formatDurationShort,
  formatMicroCents,
} from "../../../lib/api";
import { useTenantScope } from "../../../lib/auth-context";
import { useT } from "../../../lib/i18n";

const GROUP_OPTIONS: Array<{ value: string; labelKey: string }> = [
  { value: "", labelKey: "billing.groupby.none" },
  { value: "provider", labelKey: "billing.groupby.provider" },
  { value: "campaign", labelKey: "billing.groupby.campaign" },
];

function defaultRange(): { from: string; to: string } {
  const now = new Date();
  const to = now.toISOString().slice(0, 10);
  const fromDate = new Date(now);
  fromDate.setDate(fromDate.getDate() - 29);
  const from = fromDate.toISOString().slice(0, 10);
  return { from, to };
}

export default function BillingPage() {
  const tenant = useTenantScope();
  const t = useT();
  const toast = useToast();
  const [data, setData] = useState<BillingSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [groupBy, setGroupBy] = useState("");
  const initial = useMemo(defaultRange, []);
  const [from, setFrom] = useState(initial.from);
  const [to, setTo] = useState(initial.to);

  async function reload() {
    setLoading(true);
    try {
      const r = await api.billingSummary({ from, to, groupBy, tenantOverride: tenant });
      setData(r);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "error";
      toast.push(t("billing.toast.error", { err: code }), "danger");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant, groupBy, from, to]);

  const totals = data
    ? {
        calls: data.totalCalls,
        duration: data.totalDurationSec,
        cost: data.totalMicroCents,
      }
    : { calls: 0, duration: 0, cost: 0 };

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">{t("billing.eyebrow")}</p>
          <h1>{t("billing.title")}</h1>
          <p className="subtle">{t("billing.subtitle")}</p>
        </div>
      </div>

      <div className="filter-row" style={{ gap: 16, alignItems: "flex-end" }}>
        <div className="field">
          <label>{t("billing.filter.from")}</label>
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
        </div>
        <div className="field">
          <label>{t("billing.filter.to")}</label>
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)} />
        </div>
        <div className="field">
          <label>{t("billing.filter.groupby")}</label>
          <select value={groupBy} onChange={(e) => setGroupBy(e.target.value)}>
            {GROUP_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {t(o.labelKey)}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Usamos `.grid` + `.panel stat-card` como el resto de la app
       *  (StatCard component lo hace así). Sin la clase `panel` los
       *  divs quedan sin fondo y solo se ve el ::after coral suelto. */}
      <div className="grid" style={{ marginTop: 16 }}>
        <section className="panel stat-card">
          <p className="eyebrow">{t("billing.stat.calls")}</p>
          <span className="stat-value">{totals.calls}</span>
        </section>
        <section className="panel stat-card">
          <p className="eyebrow">{t("billing.stat.duration")}</p>
          <span className="stat-value">{formatDurationShort(totals.duration)}</span>
        </section>
        <section className="panel stat-card">
          <p className="eyebrow">{t("billing.stat.cost")}</p>
          <span className="stat-value">{formatMicroCents(totals.cost)}</span>
        </section>
      </div>

      {loading ? (
        <TableSkeleton cols={4} rows={6} />
      ) : !data || data.rows.length === 0 ? (
        <EmptyState
          icon={Receipt}
          title={t("billing.empty.title")}
          description={t("billing.empty.desc")}
        />
      ) : (
        <div className="table-wrap" style={{ marginTop: 16 }}>
          <table>
            <thead>
              <tr>
                <th>{t("billing.col.day")}</th>
                {groupBy ? <th>{t(`billing.col.${groupBy}`)}</th> : null}
                <th>{t("billing.col.calls")}</th>
                <th>{t("billing.col.duration")}</th>
                <th>{t("billing.col.cost")}</th>
              </tr>
            </thead>
            <tbody>
              {data.rows.map((row, idx) => (
                <tr key={`${row.day}-${row.bucketId}-${idx}`}>
                  <td>{new Date(row.day).toLocaleDateString()}</td>
                  {groupBy ? (
                    <td>
                      <span className="chip">{row.bucketLabel || row.bucketId || "—"}</span>
                    </td>
                  ) : null}
                  <td>{row.calls}</td>
                  <td>{formatDurationShort(row.durationSec)}</td>
                  <td className="primary-cell">{formatMicroCents(row.totalMicroCents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
