'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { jobFailureTitle, parseFailureReason } from '@/lib/api/failure-reason';
import { MATCH_ROW_UNPICKED_CTA, MATCH_ROW_UNPICKED_HINT } from '@/lib/clips/copy';
import { fullChipLabel, HUB_ROW_STAGE, matchMetaParts, OUTPUT_TONE, shortsChipTone, type HubMatch } from '@/lib/clips/hub';
import { flowHref, PRODUCE_FORMAT, produceHref, type ProduceFormat } from '@/lib/clips/routes';
import { matchDateLabel, prettyMapName } from '@/lib/format';
import { FLOW_PRODUCE_CTA } from '@/lib/produce/copy';
import { demoDeletePrompt, type DemoRowGroup } from '@/lib/produce/demo-rows';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { parseScore } from '@/components/matches/match-score';
import { CoverImage } from '@/components/studio/cover-image';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';

/** Cover hues a map name hashes into, so a map keeps its colour on every visit. */
const MAP_HUES = [
  { tint: 'bg-chart-1/30', text: 'text-chart-1' },
  { tint: 'bg-chart-2/30', text: 'text-chart-2' },
  { tint: 'bg-chart-3/30', text: 'text-chart-3' },
  { tint: 'bg-chart-4/30', text: 'text-chart-4' },
  { tint: 'bg-chart-5/30', text: 'text-chart-5' },
  { tint: 'bg-chart-6/30', text: 'text-chart-6' },
  { tint: 'bg-chart-7/30', text: 'text-chart-7' },
  { tint: 'bg-side-ct/30', text: 'text-side-ct' },
  { tint: 'bg-side-t/30', text: 'text-side-t' },
] as const;

function mapHue(map: string): (typeof MAP_HUES)[number] {
  let hash = 0;
  for (const char of map) hash = (hash * 31 + (char.codePointAt(0) ?? 0)) >>> 0;
  return MAP_HUES[hash % MAP_HUES.length] ?? MAP_HUES[0];
}

export function madeCount(row: HubMatch, format: ProduceFormat): number {
  return format === PRODUCE_FORMAT.short ? row.shorts.length : row.fulls.length;
}

type DemoCardProps = {
  row: DemoRowGroup;
  format: ProduceFormat;
  onChange: () => void;
  onDelete: () => Promise<void>;
};

/** One loaded demo as a card: the map cover (or the demo's thumbnail) under the map tint, then this flow's next step. */
export function DemoCard({ row, format, onChange, onDelete }: DemoCardProps): ReactNode {
  const { match, stage } = row;
  const mapName = prettyMapName(match.map);
  const parsing = stage === HUB_ROW_STAGE.parsing;
  const ready = stage === HUB_ROW_STAGE.ready;
  const made = madeCount(row, format);
  const meta = matchMetaParts(match, '').filter((part) => part !== '');

  return (
    <DemoCardFrame>
      <DemoCover map={match.map} thumbnailUrl={match.thumbnailUrl} striped={parsing} badge={<ResultBadge row={row} />} dateLabel={matchDateLabel(match)} score={match.score} />

      <div className="flex flex-1 flex-col gap-2.5 px-4 pt-3.5 pb-4">
        {meta.length > 0 ? <p className="text-body-sm wrap-anywhere text-fg-2">{meta.join(' · ')}</p> : null}
        <StateLine row={row} format={format} />
        <div className="mt-auto flex flex-wrap items-center gap-2">
          {ready ? (
            <Button asChild size="sm" className={cn('min-w-0 flex-1 shrink font-bold', FORMAT_TONE[format].fill)}>
              <Link href={produceHref(match.id, format)} className="truncate">{made === 0 ? FLOW_PRODUCE_CTA[format].first : FLOW_PRODUCE_CTA[format].again}</Link>
            </Button>
          ) : null}
          {stage === HUB_ROW_STAGE.unpicked ? (
            <Button asChild size="sm" variant="outline" className={cn('min-w-0 flex-1 shrink border-[1.5px] bg-transparent font-bold', FORMAT_TONE[format].border)}>
              <Link href={flowHref(format, { job: match.id })} className="truncate">{MATCH_ROW_UNPICKED_CTA}</Link>
            </Button>
          ) : null}
          {parsing ? <span className="flex-1 text-body-sm text-fg-3">Podrás crear al terminar</span> : null}
          {stage === HUB_ROW_STAGE.failed ? <span className="flex-1" /> : null}
          <DeleteMatchButton label={mapName} confirmText={demoDeletePrompt(row)} onConfirm={onDelete} onDeleted={onChange} messageBelow />
        </div>
      </div>
    </DemoCardFrame>
  );
}

export function DemoCardFrame({ children }: { children: ReactNode }): ReactNode {
  return (
    <article className="flex flex-col overflow-hidden rounded-2xl border border-border-subtle bg-surface-2 transition-colors duration-(--dur-fast) hover:border-border-strong">
      {children}
    </article>
  );
}

type DemoCoverProps = {
  map: string;
  thumbnailUrl?: string | undefined;
  striped?: boolean;
  badge?: ReactNode;
  dateLabel: string;
  score: string;
};

/** The card's top: the map cover (or the demo's thumbnail) under the map tint, with the map, date and score. */
export function DemoCover({ map, thumbnailUrl, striped = false, badge, dateLabel, score }: DemoCoverProps): ReactNode {
  const mapName = prettyMapName(map);
  const hue = mapHue(map);
  return (
    <div className="relative flex h-33 flex-col justify-between overflow-hidden bg-surface-3 px-4 py-3.5">
      <span aria-hidden className="absolute inset-0">
        <MapCover map={map} />
        <span className="absolute inset-0">
          <CoverImage src={thumbnailUrl} />
        </span>
      </span>
      <span aria-hidden className={cn('absolute inset-0', hue.tint)} />
      <span aria-hidden className="absolute inset-0 bg-linear-to-t from-surface-2/90 via-surface-2/30 to-surface-2/60" />
      {striped ? <span aria-hidden className="absolute inset-0 bg-[repeating-linear-gradient(-45deg,transparent_0_8px,var(--surface-5)_8px_16px)] opacity-40" /> : null}
      <span aria-hidden className={cn('pointer-events-none absolute -right-2 -bottom-9 select-none font-display text-[150px] font-black leading-none tracking-[-0.06em] opacity-15', hue.text)}>
        {mapName.charAt(0)}
      </span>
      <div className="relative flex items-center justify-between gap-2">
        {badge}
        <span className="ml-auto truncate text-meta text-fg-2">{dateLabel}</span>
      </div>
      <div className="relative flex items-baseline justify-between gap-2">
        <h3 className="min-w-0 truncate font-display text-[1.875rem] font-extrabold leading-none tracking-[-0.02em] text-fg-1">{mapName}</h3>
        <Score score={score} />
      </div>
    </div>
  );
}

function ResultBadge({ row }: { row: DemoRowGroup }): ReactNode {
  const base = 'inline-flex h-5.5 items-center rounded-md px-2 text-meta font-bold';
  if (row.stage === HUB_ROW_STAGE.parsing) return <span className={cn(base, 'bg-surface-5 text-fg-1')}>Analizando</span>;
  if (row.stage === HUB_ROW_STAGE.failed) return <span className={cn(base, 'bg-destructive-solid text-white')}>Error</span>;
  // Without a chosen player the demo has no side of its own, so no win or loss either.
  if (row.stage === HUB_ROW_STAGE.unpicked) return <span />;
  const { ours, theirs } = parseScore(row.match.score);
  if (ours === null || theirs === null) return <span />;
  if (ours > theirs) return <span className={cn(base, 'bg-success text-success-foreground')}>Victoria</span>;
  if (ours < theirs) return <span className={cn(base, 'bg-destructive-solid text-white')}>Derrota</span>;
  return <span className={cn(base, 'bg-surface-5 text-fg-1')}>Empate</span>;
}

function Score({ score }: { score: string }): ReactNode {
  const { ours, theirs } = parseScore(score);
  if (ours === null || theirs === null) return null;
  return (
    <span role="img" aria-label={`Marcador ${ours} a ${theirs}`} className="shrink-0 text-title font-extrabold tabular-nums text-fg-1">
      {ours} : {theirs}
    </span>
  );
}

/** What the demo is doing now, under its meta line. */
function StateLine({ row, format }: { row: DemoRowGroup; format: ProduceFormat }): ReactNode {
  const { match, stage } = row;
  if (stage === HUB_ROW_STAGE.parsing) {
    return (
      <span role="status" className="flex flex-col gap-2">
        <span className="flex items-center gap-2 text-meta text-fg-1">
          <span aria-hidden className="studio-spinner" />
          {match.player === undefined ? 'Analizando la demo' : `Analizando las jugadas de ${match.player}`}
        </span>
        <span className="studio-bar text-primary">
          <span className="studio-indeterminate" />
        </span>
      </span>
    );
  }
  if (stage === HUB_ROW_STAGE.unpicked) {
    return <span className="text-body-sm text-fg-2">{MATCH_ROW_UNPICKED_HINT}</span>;
  }
  if (stage === HUB_ROW_STAGE.failed) {
    return (
      <span role="status" className="flex flex-col gap-0.5">
        <span className="text-meta text-destructive">{jobFailureTitle(match.failureReason)}</span>
        <span className="text-body-sm text-fg-2">{parseFailureReason(match.failureReason, { job: true }).message}</span>
      </span>
    );
  }
  return <MadeChip row={row} format={format} />;
}

/** How many videos of this format the demo already has; nothing when there are none. */
function MadeChip({ row, format }: { row: HubMatch; format: ProduceFormat }): ReactNode {
  if (format === PRODUCE_FORMAT.full) {
    const [latest] = row.fulls;
    return latest === undefined ? null : <StatusTag tone={OUTPUT_TONE[latest.state]} className="self-start">{fullChipLabel(row.fulls)}</StatusTag>;
  }
  return row.shorts.length > 0 ? <StatusTag tone={shortsChipTone(row.shorts)} className="self-start">Shorts · {row.shorts.length}</StatusTag> : null;
}
