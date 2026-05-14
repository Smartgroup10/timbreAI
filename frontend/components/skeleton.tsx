"use client";

// Skeleton rows para tablas — mismo número de columnas que la tabla real
// para que el layout no salte cuando llega la data. Cada celda renderiza
// una barra animada con anchos pseudo-aleatorios (pero deterministas por
// índice) que dan sensación de contenido sin parpadear.

export function TableSkeleton({ cols, rows = 5 }: { cols: number; rows?: number }) {
  return (
    <div className="table-wrap">
      <table className="skeleton-table" aria-hidden="true">
        <tbody>
          {Array.from({ length: rows }).map((_, r) => (
            <tr key={r}>
              {Array.from({ length: cols }).map((_, c) => (
                <td key={c}>
                  <span
                    className="skeleton-bar"
                    style={{ width: `${widthFor(r, c)}%` }}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Patrón estable por fila+columna para que las barras no salten entre
// renders. No es aleatorio real — es suficiente para evitar uniformidad.
function widthFor(row: number, col: number): number {
  const widths = [70, 55, 80, 45, 65, 60, 75, 50];
  return widths[(row * 3 + col * 2) % widths.length];
}

// Variante para grids de tarjetas (campañas, bots, propiedades).
export function CardGridSkeleton({ count = 2 }: { count?: number }) {
  return (
    <div className="grid two" aria-hidden="true">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="panel skeleton-card">
          <div className="skeleton-bar" style={{ width: "30%", height: 10 }} />
          <div className="skeleton-bar" style={{ width: "60%", height: 22, marginTop: 10 }} />
          <div className="skeleton-bar" style={{ width: "80%", height: 10, marginTop: 14 }} />
          <div className="skeleton-bar" style={{ width: "70%", height: 10, marginTop: 6 }} />
        </div>
      ))}
    </div>
  );
}
