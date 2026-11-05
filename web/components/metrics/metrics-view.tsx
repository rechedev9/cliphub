'use client';

import { useState, type ReactNode } from 'react';
import type { MetricsReport } from '@/lib/api/metrics';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { metricsMatchIndex, metricsSections, modelText, type MetricsSectionOptions } from '@/lib/metrics/advanced';
import { SKIP_REASON_LABEL, type ReportSection } from '@/lib/reports/scout-sections';
import { SectionView } from '@/components/reports/scout-view';
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
}: {
  report: MetricsReport;
  options: MetricsSectionOptions;
  /** Known matches, so an evidence chip names its rival. */
  matches?: ReadonlyMap<string, ScoutMatchSummary>;
  /** Content shown above the sections, such as a round's win probability. */
  lead?: ReactNode;
  /** Section ids the page shows elsewhere. */
  omit?: readonly string[];
}): ReactNode {
  const sections = metricsSections(report, options).filter((s) => !omit.includes(s.id));
  const [active, setActive] = useState(sections[0]?.id ?? '');
  const section = sections.find((s) => s.id === active) ?? sections[0];
  const index = metricsMatchIndex(report, matches);
  return (
    <div className="flex flex-col gap-5">
      {lead}
      {report.skipped.length > 0 ? (
        <p className="measure-read text-body-sm text-fg-2">
          {report.skipped.length === 1 ? 'Una partida no se pudo leer' : `${report.skipped.length} partidas no se pudieron leer`}:{' '}
          {[...new Set(report.skipped.map((skip) => SKIP_TEXT[skip.reason] ?? skip.reason))].join(' ')}
        </p>
      ) : null}
      <Segmented
        label="Sección de métricas"
        value={section?.id ?? ''}
        options={sections.map((s) => ({ value: s.id, label: s.title }))}
        onChange={setActive}
      />
      {section ? <SectionBody section={section} report={report} options={options} matches={index} /> : null}
      <p className="measure-read text-meta text-fg-3">{modelText(report)}</p>
    </div>
  );
}
