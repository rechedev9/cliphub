import type { ReactNode } from 'react';
import Link from 'next/link';
import type { EquipmentIconName } from '@/lib/api/game-assets';
import { deltaArrow, heatLevel } from '@/lib/analysis/compare';
import type { Reliability } from '@/lib/analysis/reference';
import { MapThumb } from '@/components/analysis/map-thumb';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { cn } from '@/lib/utils';

export type HeatColumn = {
  readonly id: string;
  readonly header: string;
  /**
   * Tints the column's cells against `baseline`: green when better, red when
   * worse, strongest at `spread` away. Columns without it stay plain.
   */
  readonly heat?: { readonly baseline: number; readonly spread: number; readonly higherIsBetter?: boolean };
  readonly align?: 'start' | 'end';
};

export type HeatCell = {
  readonly text: string;
  /** The number the tint reads; null or absent leaves the cell untinted. */
  readonly value?: number | null;
  /** Sample size behind the value, shown in parentheses beside it. */
  readonly n?: number;
  readonly href?: string;
  readonly tone?: 'ct' | 't';
  /** A map name such as `de_mirage`: the cell leads with its radar thumbnail. */
  readonly map?: string;
  /** A weapon or grenade: the cell leads with its silhouette. */
  readonly icon?: EquipmentIconName | null;
  /** This cell's own reference, such as the library's rate for the same site; overrides the column baseline, null leaves it plain. */
  readonly reference?: number | null;
  /** Hover text for the reference, such as "Biblioteca 48 %". */
  readonly referenceText?: string;
  /** The shrunk estimate: the cell is tinted only when its interval leaves the reference out, and dimmed when the sample is weak. */
  readonly reliability?: Reliability | null;
};

export type HeatRow = { readonly id: string; readonly cells: Readonly<Record<string, HeatCell>> };

export type HeatTableProps = {
  /** Caption and accessible name. */
  label: string;
  columns: readonly HeatColumn[];
  rows: readonly HeatRow[];
  /** Below this sample a cell is dimmed and left untinted. */
  minSample: number;
  /** What n counts, plural, such as "rondas". */
  sampleNoun: string;
  empty: string;
  /** Header actions, such as a selector. */
  actions?: ReactNode;
  /** What the tint compares with; default "la media de la tabla". */
  reference?: string;
  /** The cells carry a reliability, so the caption says only reliable differences are marked. */
  reliableOnly?: boolean;
};

const WIN = ['', 'bg-outcome-win/8', 'bg-outcome-win/16', 'bg-outcome-win/24'] as const;
const LOSS = ['', 'bg-outcome-loss/8', 'bg-outcome-loss/16', 'bg-outcome-loss/24'] as const;
const TONE = { ct: 'text-side-ct', t: 'text-side-t' } as const;

function tint(level: number): string {
  if (level > 0) return WIN[Math.min(level, 3)] ?? '';
  if (level < 0) return LOSS[Math.min(-level, 3)] ?? '';
  return '';
}

function CellView({
  cell,
  column,
  first,
  minSample,
  sampleNoun,
}: {
  cell: HeatCell | undefined;
  column: HeatColumn;
  first: boolean;
  minSample: number;
  sampleNoun: string;
}): ReactNode {
  const sticky = first ? 'sticky left-0 z-10 bg-surface-2' : '';
  if (cell === undefined) return <td className={cn('px-2.5 py-1.5', sticky)} />;
  const small = cell.n !== undefined && cell.n < minSample;
  const low = small || cell.reliability?.weak === true;
  // An explicit null reference means the library is too thin here: no tint, never a silent fallback.
  const base = cell.reference === undefined ? (column.heat?.baseline ?? null) : cell.reference;
  const called = cell.reliability ? cell.reliability.differs : !low;
  const level =
    column.heat && base !== null && !small && called && cell.value !== undefined && cell.value !== null
      ? heatLevel(cell.value, base, column.heat.spread, column.heat.higherIsBetter ?? true)
      : 0;
  const title = small
    ? `Muestra pequeña: menos de ${minSample} ${sampleNoun}`
    : [cell.referenceText, cell.reliability?.detail].filter(Boolean).join('. ') || undefined;
  const arrow = level !== 0 && base !== null && cell.value !== undefined && cell.value !== null ? deltaArrow(cell.value - base) : '';
  const text = cell.href ? (
    <Link href={cell.href} className="font-medium text-fg-1 hover:text-primary hover:underline">
      {cell.text}
    </Link>
  ) : (
    cell.text
  );
  return (
    <td
      className={cn(
        'px-2.5 py-1.5 whitespace-nowrap',
        column.align === 'end' && 'text-right tabular-nums',
        low ? 'text-fg-3' : 'text-fg-1',
        cell.tone && !low && TONE[cell.tone],
        first ? sticky : tint(level),
      )}
      title={title}
    >
      <span className={cn('inline-flex items-center gap-1.5', column.align === 'end' && 'justify-end')}>
        {cell.map ? <MapThumb map={cell.map} className="h-5 w-8" crop={1.5} /> : null}
        {cell.icon ? <EquipmentIcon name={cell.icon} height={12} className="text-fg-2" /> : null}
        {text}
        {arrow === '' ? null : (
          <span aria-hidden className={cn('text-[0.7em]', level > 0 ? 'text-outcome-win' : 'text-outcome-loss')}>
            {arrow}
          </span>
        )}
        {cell.n === undefined ? null : (
          <>
            <span aria-hidden className="text-meta text-fg-3 tabular-nums">
              ({cell.n})
            </span>
            <span className="sr-only">, {cell.n} {sampleNoun}</span>
          </>
        )}
      </span>
    </td>
  );
}

/**
 * A dense table whose numbers are read against a baseline, "52 % ▲ (24)":
 * tint and arrow say better or worse, n in parentheses the sample, so a strong
 * tint on three rounds is never mistaken for a trend. Cells under `minSample`
 * are dimmed and never tinted. Wide tables scroll inside with the first column
 * pinned, so nothing is cut at the right edge.
 */
export function HeatTable({
  label,
  columns,
  rows,
  minSample,
  sampleNoun,
  empty,
  actions,
  reference = 'la media de la tabla',
  reliableOnly = false,
}: HeatTableProps): ReactNode {
  return (
    <section aria-label={label} className="studio-panel flex min-w-0 flex-col">
      <header className="flex min-h-9 flex-wrap items-center justify-between gap-2 border-b border-border px-3 py-1.5">
        <h2 className="text-body-sm font-semibold text-fg-1">{label}</h2>
        {actions ? <div className="flex items-center gap-2">{actions}</div> : null}
      </header>
      {rows.length === 0 ? (
        <p className="px-3 py-2 text-body-sm text-fg-2">{empty}</p>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label={label} className="w-full border-collapse text-left text-body-sm">
            <thead className="bg-surface-2 text-meta text-fg-3">
              <tr className="border-b border-border">
                {columns.map((column, index) => (
                  <th
                    key={column.id}
                    scope="col"
                    className={cn(
                      'h-8 px-2.5 font-medium whitespace-nowrap',
                      column.align === 'end' && 'text-right',
                      index === 0 && 'sticky left-0 z-10 bg-surface-2',
                    )}
                  >
                    {column.header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.id} className="border-b border-border-subtle last:border-b-0">
                  {columns.map((column, index) => (
                    <CellView
                      key={column.id}
                      cell={row.cells[column.id]}
                      column={column}
                      first={index === 0}
                      minSample={minSample}
                      sampleNoun={sampleNoun}
                    />
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <p className="border-t border-border-subtle px-3 py-1.5 text-meta text-fg-3">
        ▲▼ y color frente a {reference}{reliableOnly ? ', solo cuando la diferencia es fiable' : ''}; entre paréntesis, las {sampleNoun}. Con
        menos de {minSample} {sampleNoun}{reliableOnly ? ' o poca muestra' : ''}, en gris.
      </p>
    </section>
  );
}
