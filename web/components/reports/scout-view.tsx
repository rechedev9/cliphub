'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Clapperboard } from 'lucide-react';
import type { ScoutEvidence, ScoutMatchSummary, ScoutReport } from '@/lib/api/scout';
import {
  evidenceLabel,
  evidencePath,
  evidenceTitle,
  matchIndex,
  scoutSections,
  thresholdsText,
  type ReportCell,
  type ReportSection,
  type ReportTable,
} from '@/lib/reports/scout-sections';
import { MapThumb } from '@/components/analysis/map-thumb';
import { RecordTabs } from '@/components/records/record-tabs';
import { RelatedList } from '@/components/records/related-list';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { cn } from '@/lib/utils';

/** Opens the video dialog on the example rounds of one table row. */
export type RowVideoHandler = (evidence: readonly ScoutEvidence[]) => void;

function rowEvidence(row: readonly ReportCell[]): ScoutEvidence[] {
  return row.flatMap((cell) => cell.evidence ?? []);
}

const TONE_CLASS = {
  win: 'text-outcome-win',
  loss: 'text-outcome-loss',
  ct: 'text-side-ct',
  t: 'text-side-t',
} as const;

function CellView({ cell, matches, minSample }: { cell: ReportCell; matches: ReadonlyMap<string, ScoutMatchSummary>; minSample: number }): ReactNode {
  const className = cn(
    'px-2.5 py-1.5 align-top',
    cell.numeric && 'text-right whitespace-nowrap',
    cell.lowSample && 'text-fg-3 italic',
    cell.tone && TONE_CLASS[cell.tone],
  );
  if (cell.evidence && cell.evidence.length > 0) {
    return (
      <td className={className}>
        <span className="flex flex-wrap gap-1">
          {cell.evidence.map((evidence) => (
            <Link
              key={`${evidence.match_id}:${evidence.round}:${evidence.tick}`}
              href={evidencePath(evidence)}
              title={evidenceTitle(evidence, matches)}
              className="rounded-sm border border-border px-1.5 text-meta whitespace-nowrap text-fg-2 hover:border-primary hover:text-primary"
            >
              {evidenceLabel(evidence, matches)}
            </Link>
          ))}
        </span>
      </td>
    );
  }
  return (
    <td className={className} title={cell.lowSample ? `Muestra pequeña: menos de ${minSample} rondas` : undefined}>
      <span className={cn('inline-flex items-center gap-1.5', cell.numeric && 'justify-end')}>
        {cell.map ? <MapThumb map={cell.map} className="h-5 w-8" crop={1.5} /> : null}
        {cell.icon ? <EquipmentIcon name={cell.icon} height={12} className="text-fg-2" /> : null}
        {cell.href ? (
          <Link href={cell.href} className="font-medium text-fg-1 hover:text-primary hover:underline">
            {cell.text}
          </Link>
        ) : (
          cell.text
        )}
      </span>
    </td>
  );
}

function TableView({
  table,
  matches,
  minSample,
  onRowVideo,
}: {
  table: ReportTable;
  matches: ReadonlyMap<string, ScoutMatchSummary>;
  minSample: number;
  onRowVideo?: RowVideoHandler;
}): ReactNode {
  const withVideo = onRowVideo !== undefined && table.rows.some((row) => rowEvidence(row).length > 0);
  return (
    <RelatedList title={table.caption} count={table.rows.length}>
      {table.rows.length === 0 ? (
        <p className="px-4 py-3 text-body-sm text-fg-2">{table.empty}</p>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label={table.caption} className="w-full border-collapse text-left text-body-sm">
            <thead className="bg-surface-2 text-meta text-fg-3">
              <tr className="border-b border-border">
                {table.columns.map((column) => (
                  <th key={column} scope="col" className="h-8 px-2.5 font-medium whitespace-nowrap">
                    {column}
                  </th>
                ))}
                {withVideo ? (
                  <th scope="col" className="h-8 px-2.5 font-medium whitespace-nowrap">
                    Vídeo
                  </th>
                ) : null}
              </tr>
            </thead>
            <tbody>
              {table.rows.map((row, index) => (
                <tr key={index} className="border-b border-border-subtle last:border-b-0">
                  {row.map((cell, column) => (
                    <CellView key={column} cell={cell} matches={matches} minSample={minSample} />
                  ))}
                  {withVideo ? (
                    <td className="px-2.5 py-1.5 align-top">
                      {rowEvidence(row).length > 0 ? (
                        <Button type="button" size="xs" variant="ghost" onClick={() => onRowVideo?.(rowEvidence(row))}>
                          <Clapperboard aria-hidden />
                          Vídeo
                        </Button>
                      ) : null}
                    </td>
                  ) : null}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </RelatedList>
  );
}

/** One report section: its intro, its facts and its tables of evidence. */
export function SectionView({
  section,
  matches,
  minSample,
  onRowVideo,
}: {
  section: ReportSection;
  matches: ReadonlyMap<string, ScoutMatchSummary>;
  minSample: number;
  onRowVideo?: RowVideoHandler;
}): ReactNode {
  return (
    <div className="flex flex-col gap-3">
      <p className="measure-read text-body-sm text-fg-2">{section.intro}</p>
      {section.facts.length > 0 ? (
        <dl className="flex flex-wrap gap-x-6 gap-y-1.5">
          {section.facts.map((fact) => (
            <div key={fact.label} className="flex flex-col gap-0.5">
              <dt className="text-meta text-fg-3">{fact.label}</dt>
              <dd className="text-body-sm text-fg-1">{fact.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {section.tables.map((table) => (
        <TableView key={table.caption} table={table} matches={matches} minSample={minSample} onRowVideo={onRowVideo} />
      ))}
    </div>
  );
}

/**
 * Every section of a scouting report as record tabs. `leading` adds tabs
 * before the sections, such as the report's notes.
 */
export function ScoutView({
  scout,
  leading = [],
  onRowVideo,
  tab,
  onTabChange,
  actions,
}: {
  scout: ScoutReport;
  leading?: readonly { id: string; label: string; content: ReactNode }[];
  /** When set, every row with example rounds gets a video button. */
  onRowVideo?: RowVideoHandler;
  /** The open tab, when the page needs to know it. */
  tab?: string;
  onTabChange?: (tab: string) => void;
  /** Buttons at the right end of the section tabs. */
  actions?: ReactNode;
}): ReactNode {
  const matches = matchIndex(scout);
  const minSample = scout.thresholds.min_reliable_sample;
  const sections = scoutSections(scout);
  return (
    <div className="flex flex-col gap-3">
      <RecordTabs
        value={tab}
        onValueChange={onTabChange}
        actions={actions}
        tabs={[
          ...leading,
          ...sections.map((section) => ({
            id: section.id,
            label: section.title,
            content: (
              <div className="flex flex-col gap-3">
                <SectionView section={section} matches={matches} minSample={minSample} onRowVideo={onRowVideo} />
                <p className="measure-read text-meta text-fg-3">{thresholdsText(scout)}</p>
              </div>
            ),
          })),
        ]}
      />
    </div>
  );
}
