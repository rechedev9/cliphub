import type { ReactNode } from 'react';
import Link from 'next/link';
import { heatLevel } from '@/lib/analysis/compare';
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
  /** Sample size behind the value, shown small beside it. */
  readonly n?: number;
  readonly href?: string;
  readonly tone?: 'ct' | 't';
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
};

const WIN = ['', 'bg-outcome-win/10', 'bg-outcome-win/20', 'bg-outcome-win/30'] as const;
const LOSS = ['', 'bg-outcome-loss/10', 'bg-outcome-loss/20', 'bg-outcome-loss/30'] as const;
const TONE = { ct: 'text-side-ct', t: 'text-side-t' } as const;

function tint(level: number): string {
  if (level > 0) return WIN[Math.min(level, 3)] ?? '';
  if (level < 0) return LOSS[Math.min(-level, 3)] ?? '';
  return '';
}

function CellView({ cell, column, minSample, sampleNoun }: { cell: HeatCell | undefined; column: HeatColumn; minSample: number; sampleNoun: string }): ReactNode {
  if (cell === undefined) return <td className="px-3 py-2" />;
  const low = cell.n !== undefined && cell.n < minSample;
  const level =
    column.heat && !low && cell.value !== undefined && cell.value !== null
      ? heatLevel(cell.value, column.heat.baseline, column.heat.spread, column.heat.higherIsBetter ?? true)
      : 0;
  const content = cell.href ? (
    <Link href={cell.href} className="font-medium text-fg-1 hover:text-primary hover:underline">
      {cell.text}
    </Link>
  ) : (
    cell.text
  );
  return (
    <td
      className={cn(
        'px-3 py-2 whitespace-nowrap',
        column.align === 'end' && 'text-right tabular-nums',
        low ? 'text-fg-3' : 'text-fg-1',
        cell.tone && !low && TONE[cell.tone],
        tint(level),
      )}
      title={low ? `Muestra pequeña: menos de ${minSample} ${sampleNoun}` : undefined}
    >
      {content}
      {cell.n === undefined ? null : (
        <>
          <span aria-hidden className="ml-1.5 text-meta text-fg-3 tabular-nums">
            {cell.n}
          </span>
          <span className="sr-only">, {cell.n} {sampleNoun}</span>
        </>
      )}
    </td>
  );
}

/**
 * A table whose numbers are tinted against a baseline, with the sample size
 * beside every number so a strong tint on three rounds is never mistaken for
 * a trend: cells under `minSample` are dimmed and never tinted.
 */
export function HeatTable({ label, columns, rows, minSample, sampleNoun, empty, actions }: HeatTableProps): ReactNode {
  return (
    <section aria-label={label} className="studio-panel flex flex-col">
      <header className="flex min-h-11 flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-2">
        <h2 className="text-body-sm font-semibold text-fg-1">{label}</h2>
        {actions ? <div className="flex items-center gap-2">{actions}</div> : null}
      </header>
      {rows.length === 0 ? (
        <p className="px-4 py-3 text-body-sm text-fg-2">{empty}</p>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label={label} className="w-full border-collapse text-left text-body-sm">
            <thead className="bg-surface-2 text-fg-3">
              <tr className="border-b border-border">
                {columns.map((column) => (
                  <th key={column.id} scope="col" className={cn('h-9 px-3 font-medium whitespace-nowrap', column.align === 'end' && 'text-right')}>
                    {column.header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.id} className="border-b border-border-subtle last:border-b-0">
                  {columns.map((column) => (
                    <CellView key={column.id} cell={row.cells[column.id]} column={column} minSample={minSample} sampleNoun={sampleNoun} />
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <p className="border-t border-border-subtle px-4 py-2 text-meta text-fg-3">
        La cifra pequeña son las {sampleNoun} de cada dato. Verde es mejor que la media de la tabla y rojo, peor; con menos de {minSample}{' '}
        {sampleNoun} el dato sale en gris y sin color.
      </p>
    </section>
  );
}
