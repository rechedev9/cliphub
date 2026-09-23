'use client';

import { useState, type PointerEvent, type ReactNode } from 'react';
import type { FaceitMatch, FaceitMatchStats } from '@/lib/api/faceit';
import { faceitTrend, summarizeFaceitMatches, type FaceitTrend } from '@/lib/faceit-stats';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { cn } from '@/lib/utils';

const RESULT = {
  win: { label: 'Victoria', text: 'text-success', bar: 'bottom-[calc(50%+1px)] top-0.5 rounded-t-[2px] bg-success' },
  loss: { label: 'Derrota', text: 'text-destructive', bar: 'top-[calc(50%+1px)] bottom-0.5 rounded-b-[2px] bg-destructive' },
  unknown: { label: 'Sin resultado', text: 'text-fg-2', bar: 'top-1/2 h-px bg-fg-4' },
} as const;

type Measure = {
  label: string;
  value: number | undefined;
  trend: FaceitTrend;
  format: (value: number) => string;
};

export function PlayerPerformance({ matches }: { matches: FaceitMatch[] }): ReactNode {
  // One pointed-at match for every tile: hovering a game in one trend shows that game in all four.
  const [hovered, setHovered] = useState<number | null>(null);
  const stats = summarizeFaceitMatches(matches);
  const chronological = [...matches].reverse();
  const focus = hovered === null ? undefined : chronological[hovered];
  const measures: Measure[] = [
    { label: 'K/D', value: stats.kd, trend: faceitTrend(matches, (row) => row.kd_ratio), format: (value) => value.toFixed(2) },
    { label: 'ADR', value: stats.adr, trend: faceitTrend(matches, (row) => row.adr), format: (value) => String(Math.round(value)) },
    { label: 'Headshots', value: stats.headshots, trend: faceitTrend(matches, (row) => row.headshots_percent), format: (value) => `${Math.round(value)}%` },
  ];

  return (
    <section aria-labelledby="player-performance-heading">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 text-body-sm">
        <h3 id="player-performance-heading" className="font-semibold text-fg-1">Rendimiento</h3>
        {/* The pointed-at match is named once here; the tiles only carry its value, which keeps them one line wide. */}
        <p className="min-w-0 truncate text-fg-3">{focus ? <MatchContext match={focus} /> : `Últimas ${matches.length} partidas`}</p>
      </div>
      <dl className="grid grid-cols-2 gap-3 @[48rem]/content:grid-cols-4">
        <Metric label="Victorias" value={stats.winRate === undefined ? '—' : `${stats.winRate}%`}
          detail={focus ? <><ResultWord result={focus.stats?.result} />{score(focus) ? ` ${score(focus)}` : ''}</>
            : `${stats.wins} victorias · ${stats.losses} derrotas`}>
          <ResultStrip matches={chronological} hovered={hovered} onHover={setHovered} />
        </Metric>
        {measures.map((measure) => (
          <Metric key={measure.label} label={measure.label} value={measure.value === undefined ? '—' : measure.format(measure.value)}
            detail={<MeasureDetail measure={measure} hovered={hovered} />}>
            <Sparkline trend={measure.trend} average={measure.value} hovered={hovered} onHover={setHovered} />
          </Metric>
        ))}
      </dl>
      {stats.unknown > 0 ? <p className="mt-2 text-meta tracking-normal text-fg-3">{stats.unknown} partidas sin resultado disponible, excluidas del porcentaje de victorias.</p> : null}
    </section>
  );
}

function Metric({ label, value, detail, children }: { label: string; value: string; detail: ReactNode; children: ReactNode }): ReactNode {
  return (
    <div className="studio-panel flex min-w-0 flex-col p-3.5">
      <dt className="text-body-sm text-fg-3">{label}</dt>
      <dd className="mt-1 text-stat font-semibold text-fg-1">{value}</dd>
      <dd className="mt-1.5 text-meta tracking-normal text-fg-2">{detail}</dd>
      {/* The chart only restates the table below per match, so it stays out of the accessibility tree. */}
      <dd aria-hidden className="mt-auto pt-3">{children}</dd>
    </div>
  );
}

function MeasureDetail({ measure, hovered }: { measure: Measure; hovered: number | null }): ReactNode {
  const { values, min, max } = measure.trend;
  if (hovered !== null) {
    const value = values[hovered];
    return <><span className="font-semibold text-fg-1">{value === undefined ? '—' : measure.format(value)}</span> en esta partida</>;
  }
  if (min === undefined || max === undefined) return 'Sin datos';
  return `Máx ${measure.format(max)} · Mín ${measure.format(min)}`;
}

function score(match: FaceitMatch): string {
  return match.score.for !== undefined && match.score.against !== undefined ? `${match.score.for}–${match.score.against}` : '';
}

function MatchContext({ match }: { match: FaceitMatch }): ReactNode {
  const map = prettyMapName(match.stats?.map ?? '') || 'Sin mapa';
  return <><span className="font-semibold text-fg-1">{map}</span>{match.finished_at ? ` · ${formatShortDate(match.finished_at)}` : ''}</>;
}

function ResultWord({ result }: { result?: FaceitMatchStats['result'] }): ReactNode {
  const tone = RESULT[result ?? 'unknown'];
  return <span className={cn('font-semibold', tone.text)}>{tone.label}</span>;
}

/** Last results as up/down bars around a midline, oldest left: polarity is in the direction, not only the colour. */
function ResultStrip({ matches, hovered, onHover }: { matches: FaceitMatch[]; hovered: number | null; onHover: (index: number | null) => void }): ReactNode {
  return (
    <div className="flex h-8 gap-0.5" onPointerLeave={() => onHover(null)}>
      {matches.map((match, index) => (
        <span key={match.id} onPointerEnter={() => onHover(index)} className="relative h-full min-w-0 flex-1">
          <span className={cn('absolute inset-x-0 transition-opacity duration-(--dur-fast)', RESULT[match.stats?.result ?? 'unknown'].bar,
            hovered !== null && hovered !== index && 'opacity-35')} />
        </span>
      ))}
    </div>
  );
}

/** Per-match trend, oldest left. The dashed line is the tile's average, so every point reads as above or below it. */
function Sparkline({ trend, average, hovered, onHover }: {
  trend: FaceitTrend;
  average: number | undefined;
  hovered: number | null;
  onHover: (index: number | null) => void;
}): ReactNode {
  const { values, min, max } = trend;
  if (min === undefined || max === undefined) return <div className="h-8" />;
  const count = values.length;
  const span = max - min || 1;
  const x = (index: number): number => (count === 1 ? 50 : (index / (count - 1)) * 100);
  const y = (value: number): number => 88 - ((value - min) / span) * 76;
  let path = '';
  let drawing = false;
  values.forEach((value, index) => {
    if (value === undefined) { drawing = false; return; }
    path += `${drawing ? 'L' : 'M'}${x(index).toFixed(2)} ${y(value).toFixed(2)}`;
    drawing = true;
  });
  const markerIndex = hovered ?? values.findLastIndex((value) => value !== undefined);
  const markerValue = values[markerIndex];

  function point(event: PointerEvent<HTMLDivElement>): void {
    const box = event.currentTarget.getBoundingClientRect();
    const ratio = Math.min(Math.max((event.clientX - box.left) / box.width, 0), 1);
    onHover(Math.round(ratio * (count - 1)));
  }

  return (
    <div className="relative h-8" onPointerMove={point} onPointerLeave={() => onHover(null)}>
      <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="absolute inset-0 size-full overflow-visible">
        {average !== undefined ? <line x1="0" x2="100" y1={y(average)} y2={y(average)} stroke="var(--border)"
          strokeWidth="1" strokeDasharray="3 3" vectorEffect="non-scaling-stroke" /> : null}
        <path d={path} fill="none" stroke="var(--fg-4)" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
      </svg>
      {markerValue !== undefined ? <span className="pointer-events-none absolute size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary ring-2 ring-surface-2"
        style={{ left: `${x(markerIndex)}%`, top: `${y(markerValue)}%` }} /> : null}
    </div>
  );
}
