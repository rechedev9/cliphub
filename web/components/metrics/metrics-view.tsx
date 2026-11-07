'use client';

import { useState, type ReactNode } from 'react';
import type { MetricsReport } from '@/lib/api/metrics';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { metricsMatchIndex, metricsSections, modelText, type MetricsSectionOptions } from '@/lib/metrics/advanced';
import { SKIP_REASON_LABEL, type ReportSection } from '@/lib/reports/scout-sections';
import { SectionView } from '@/components/reports/scout-view';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';
import { Segmented } from '@/components/ui/segmented';
import { BuyHeatTable, MapHeatTable } from './metrics-heat';

const SKIP_TEXT: Record<string, string> = SKIP_REASON_LABEL;

/**
 * One section. Per buy and per map read as heat tables, since their rows are
 * comparisons with very different sample sizes. The rest are the shared
 * report tables.
 */
function SectionBody({
  section,
  report,
  options,
  matches,
}: {
  section: ReportSection;
  report: MetricsReport;
  options: MetricsSectionOptions;
  matches: ReadonlyMap<string, ScoutMatchSummary>;
}): ReactNode {
  if (section.id === 'metricas-compra' || section.id === 'metricas-mapas') {
    return (
      <div className="flex flex-col gap-4">
        <p className="measure-read text-body-sm text-fg-2">{section.intro}</p>
        {section.id === 'metricas-compra' ? <BuyHeatTable report={report} options={options} /> : <MapHeatTable report={report} />}
      </div>
    );
  }
  return <SectionView section={section} matches={matches} minSample={0} />;
}

/**
 * The advanced metrics as one section at a time, picked from a segmented row,
 * with the model the numbers came from underneath.
 */
export function MetricsView({
  report,
  options,
  matches,
  lead,
  omit = [],
  leading = [],
  title,
}: {
  report: MetricsReport;
  options: MetricsSectionOptions;
  /** Known matches, so an evidence chip names its rival. */
  matches?: ReadonlyMap<string, ScoutMatchSummary>;
  /** Content shown above the sections, such as a round's win probability. */
  lead?: ReactNode;
  /** Section ids the page shows elsewhere. */
  omit?: readonly string[];
  /** Views picked before the sections, such as one match's table of every player. */
  leading?: readonly { id: string; label: string; content: ReactNode }[];
  /** A heading beside the section picker. */
  title?: string;
}): ReactNode {
  const sections = metricsSections(report, options).filter((s) => !omit.includes(s.id));
  const [active, setActive] = useState(leading[0]?.id ?? sections[0]?.id ?? '');
  const view = leading.find((v) => v.id === active);
  const section = view ? undefined : (sections.find((s) => s.id === active) ?? sections[0]);
  const index = metricsMatchIndex(report, matches);
  const picker = (
    <Segmented
      label="Sección de métricas"
      value={view?.id ?? section?.id ?? ''}
      options={[...leading.map((v) => ({ value: v.id, label: v.label })), ...sections.map((s) => ({ value: s.id, label: s.title }))]}
      onChange={setActive}
    />
  );
  return (
    <div className="flex flex-col gap-5">
      {lead}
      {report.skipped.length > 0 ? (
        <p className="measure-read text-body-sm text-fg-2">
          {report.skipped.length === 1 ? 'Una partida no se pudo leer' : `${report.skipped.length} partidas no se pudieron leer`}:{' '}
          {[...new Set(report.skipped.map((skip) => SKIP_TEXT[skip.reason] ?? skip.reason))].join(' ')}
        </p>
      ) : null}
      {title ? (
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="flex-[1_1_12.5rem] text-title font-semibold text-fg-1">{title}</h2>
          {picker}
        </div>
      ) : (
        picker
      )}
      {view?.content}
      {section ? <SectionBody section={section} report={report} options={options} matches={index} /> : null}
      <details className="measure-read text-meta text-fg-3">
        <DisclosureSummary className="min-h-10 text-body-sm text-fg-2">Cómo se calcula la probabilidad</DisclosureSummary>
        <p className="pb-2">{modelText(report)}</p>
      </details>
    </div>
  );
}
