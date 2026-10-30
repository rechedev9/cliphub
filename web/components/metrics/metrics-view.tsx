'use client';

import { useState, type ReactNode } from 'react';
import type { MetricsReport } from '@/lib/api/metrics';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { metricsMatchIndex, metricsSections, modelText, type MetricsSectionOptions } from '@/lib/metrics/advanced';
import { SKIP_REASON_LABEL } from '@/lib/reports/scout-sections';
import { SectionView } from '@/components/reports/scout-view';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';

const SKIP_TEXT: Record<string, string> = SKIP_REASON_LABEL;

/**
 * The advanced metrics as one section at a time, picked from a segmented row,
 * with the model the numbers came from underneath.
 */
export function MetricsView({
  report,
  options,
  matches,
  lead,
}: {
  report: MetricsReport;
  options: MetricsSectionOptions;
  /** Known matches, so an evidence chip names its rival. */
  matches?: ReadonlyMap<string, ScoutMatchSummary>;
  /** Content shown above the sections, such as a round's win probability. */
  lead?: ReactNode;
}): ReactNode {
  const sections = metricsSections(report, options);
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
      {section ? <SectionView section={section} matches={index} minSample={0} /> : null}
      <p className="measure-read text-meta text-fg-3">{modelText(report)}</p>
    </div>
  );
}
