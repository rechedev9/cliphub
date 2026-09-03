'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ChevronRight, Plus } from 'lucide-react';
import { api } from '@/lib/api';
import {
  MATCH_ROW_FIRST_CLIP_CTA,
  MATCH_ROW_UNPICKED_CTA,
  MATCH_ROW_UNPICKED_HINT,
  MATCH_ROW_UNPICKED_TITLE,
} from '@/lib/clips/copy';
import {
  fullChipLabel,
  HUB_NEXT_STEP,
  HUB_ROW_STAGE,
  hubNextStep,
  matchMetaParts,
  OUTPUT_STATE,
  OUTPUT_TONE,
  pluralShorts,
  roundsFromScore,
  shortsChipTone,
  type HubMatch,
} from '@/lib/clips/hub';
import { newDemoHref, PRODUCE_FORMAT, produceHref } from '@/lib/clips/routes';
import { matchDateLabel, prettyMapName } from '@/lib/format';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { parseScore } from '@/components/matches/match-score';
import { CoverImage } from '@/components/studio/cover-image';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { OutputItem } from '@/components/clips-hub/output-item';

export function matchRowId(matchId: string): string {
  return `partida-${matchId}`;
}

export type MatchRowProps = {
  row: HubMatch;
  open: boolean;
  onToggle: () => void;
  onChange: () => void;
};

/** One partida: collapsed scoreboard header, expanded Shorts + Full POV columns. */
export function MatchRow({ row, open, onToggle, onChange }: MatchRowProps): ReactNode {
  const { match, stage, shorts, fulls } = row;
  const { ours, theirs } = parseScore(match.score);
  const hasScore = ours !== null && theirs !== null;
  const win = hasScore && ours > theirs;
  const loss = hasScore && ours < theirs;
  const player = match.player;
  const expandable = stage === HUB_ROW_STAGE.ready;
  const expanded = open && expandable;
  const nextStep = hubNextStep(row);

  let bar = 'bg-border-strong';
  if (expandable && win) bar = 'bg-success';
  else if (expandable && loss) bar = 'bg-destructive';

  let headerBlock: ReactNode;
  if (stage === HUB_ROW_STAGE.parsing) headerBlock = <ParsingBlock player={player} />;
  else if (stage === HUB_ROW_STAGE.unpicked) headerBlock = <UnpickedBlock />;
  else headerBlock = <ReadyHeaderBlock hasScore={hasScore} ours={ours} theirs={theirs} win={win} loss={loss} shorts={shorts} fulls={fulls} />;

  return (
    <article
      id={matchRowId(match.id)}
      className={cn('studio-panel studio-enter flex flex-col overflow-hidden rounded-[10px]', expanded && 'studio-panel-raised')}
    >
      {/* Narrow content: the action cluster wraps under the header instead of overlaying the title. */}
      <div className="flex w-full flex-wrap items-stretch @[44rem]/content:flex-nowrap">
        <button
          type="button"
          aria-expanded={expanded}
          aria-disabled={!expandable || undefined}
          onClick={() => {
            if (expandable) onToggle();
          }}
          className={cn(
            'flex min-w-0 flex-1 flex-wrap items-stretch gap-4 py-3 pr-3 pl-[18px] text-left transition-colors duration-(--dur-fast) @[44rem]/content:flex-nowrap',
            expandable ? 'cursor-pointer hover:bg-surface-3' : 'cursor-default',
          )}
        >
          <span aria-hidden className={cn('w-1 shrink-0 self-stretch', bar)} />

          <span aria-hidden className="relative h-[47px] w-[84px] shrink-0 self-center overflow-hidden border border-border-strong">
            <MapCover map={match.map} />
            <span className="absolute inset-0">
              <CoverImage src={match.thumbnailUrl} />
            </span>
          </span>

          <span className="flex min-w-[160px] flex-1 flex-col justify-center gap-1">
            <span className="truncate font-display text-body-lg font-bold uppercase text-fg-1">{prettyMapName(match.map)}</span>
            <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
              {matchMetaParts(match, matchDateLabel(match)).join(' · ')}
            </span>
          </span>

          {headerBlock}

          {expandable ? (
            <ChevronRight
              aria-hidden
              className={cn('size-4 shrink-0 self-center text-fg-3 transition-transform duration-(--dur-fast)', expanded && 'rotate-90')}
            />
          ) : null}
        </button>

        <span className="flex w-full shrink-0 items-center justify-end gap-2 px-[18px] pb-3 @[44rem]/content:w-auto @[44rem]/content:pl-0 @[44rem]/content:py-3">
          {nextStep === HUB_NEXT_STEP.pick ? (
            <Button asChild size="xs" variant="outline-primary">
              <Link href={newDemoHref({ job: match.id })}>{MATCH_ROW_UNPICKED_CTA}</Link>
            </Button>
          ) : null}
          {nextStep === HUB_NEXT_STEP.firstClip ? (
            <Button asChild size="xs" variant="outline-primary">
              <Link href={produceHref(match.id, PRODUCE_FORMAT.short)}>
                <Plus aria-hidden />
                {MATCH_ROW_FIRST_CLIP_CTA}
              </Link>
            </Button>
          ) : null}
          <DeleteMatchButton
            label={prettyMapName(match.map)}
            onConfirm={() => api.deleteMatch(match.id)}
            onDeleted={onChange}
          />
        </span>
      </div>

      {expanded ? (
        <div className="studio-enter grid grid-cols-1 border-t border-border-subtle @[44rem]/content:grid-cols-2">
          <ShortsColumn row={row} onChange={onChange} />
          <FullColumn row={row} onChange={onChange} />
        </div>
      ) : null}
    </article>
  );
}

function ParsingBlock({ player }: { player?: string }): ReactNode {
  return (
    <span role="status" className="flex w-full shrink-0 flex-col justify-center gap-1.5 self-center @[44rem]/content:w-[280px]">
      <span className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-primary">
        <span aria-hidden className="studio-spinner" />
        {player === undefined ? 'Parseando la demo' : `Parseando POV de ${player}`}
      </span>
      <span className="studio-bar text-primary">
        <span className="studio-indeterminate" />
      </span>
      <span className="font-mono text-meta uppercase tracking-wider text-fg-3">parseando la demo · highlights al terminar</span>
    </span>
  );
}

function ReadyHeaderBlock({
  hasScore,
  ours,
  theirs,
  win,
  loss,
  shorts,
  fulls,
}: {
  hasScore: boolean;
  ours: number | null;
  theirs: number | null;
  win: boolean;
  loss: boolean;
  shorts: HubMatch['shorts'];
  fulls: HubMatch['fulls'];
}): ReactNode {
  return (
    <>
      {hasScore ? (
        <span
          role="img"
          aria-label={`Marcador ${ours} a ${theirs}`}
          className="self-center font-mono text-title font-bold leading-none tabular-nums"
        >
          <span className={cn(win && 'text-success', loss && 'text-destructive', !win && !loss && 'text-fg-1')}>{ours}</span>
          <span className="text-fg-4"> : </span>
          <span className="text-fg-1">{theirs}</span>
        </span>
      ) : null}
      {/* An empty chip says nothing; the row's CTA carries "nothing yet". */}
      {shorts.length > 0 || fulls[0] !== undefined ? (
        <span className="flex items-center gap-2 self-center">
          {shorts.length > 0 ? <StatusTag tone={shortsChipTone(shorts)}>Shorts · {shorts.length}</StatusTag> : null}
          {fulls[0] !== undefined ? (
            <StatusTag tone={OUTPUT_TONE[fulls[0].state]}>
              {fulls[0].state === OUTPUT_STATE.rec ? (
                <span aria-hidden className="neon-pulse size-1.5 shrink-0 rounded-full bg-current shadow-[0_0_6px_currentColor]" />
              ) : null}
              {fullChipLabel(fulls)}
            </StatusTag>
          ) : null}
        </span>
      ) : null}
    </>
  );
}

function UnpickedBlock(): ReactNode {
  return (
    <span className="flex w-full shrink-0 flex-col justify-center gap-1.5 self-center @[44rem]/content:w-[280px]">
      <span className="font-mono text-meta uppercase tracking-wider text-warning">{MATCH_ROW_UNPICKED_TITLE}</span>
      <span className="font-mono text-meta uppercase tracking-wider text-fg-3">{MATCH_ROW_UNPICKED_HINT}</span>
    </span>
  );
}

function ColumnHead({ label, accent, trailing }: { label: string; accent?: boolean; trailing: string }): ReactNode {
  return (
    <div className="flex justify-between font-mono text-meta uppercase tracking-widest text-fg-3">
      <span className={accent ? 'text-primary' : 'text-fg-2'}>{label}</span>
      <span>{trailing}</span>
    </div>
  );
}

function ShortsColumn({ row, onChange }: { row: HubMatch; onChange: () => void }): ReactNode {
  return (
    <div className="flex flex-col gap-2 border-border-subtle px-4 py-3 @[44rem]/content:border-r">
      <ColumnHead label="Shorts" accent trailing={pluralShorts(row.shorts.length)} />
      {row.shorts.map((output) => (
        <OutputItem key={output.id} output={output} matchId={row.match.id} onChange={onChange} />
      ))}
      <Button asChild variant="ghost" className="h-9 border border-dashed border-primary/50 font-display text-meta font-semibold uppercase tracking-wide text-primary hover:bg-primary/8 hover:text-primary">
        <Link href={produceHref(row.match.id, PRODUCE_FORMAT.short)}>
          <Plus aria-hidden />
          Clipear otro short
        </Link>
      </Button>
    </div>
  );
}

function FullColumn({ row, onChange }: { row: HubMatch; onChange: () => void }): ReactNode {
  const latest = row.fulls[0];
  const rounds = roundsFromScore(row.match.score);
  return (
    <div className="flex flex-col gap-2 px-4 py-3">
      <ColumnHead label="Full POV" trailing={latest === undefined ? 'sin generar' : fullChipLabel(row.fulls).replace('Full POV · ', '')} />
      {row.fulls.map((output) => (
        <OutputItem key={output.id} output={output} matchId={row.match.id} onChange={onChange} />
      ))}
      {latest === undefined ? (
        <div className="flex flex-col gap-2.5 rounded-lg border border-border-subtle bg-surface-1 p-3.5">
          <p className="text-label text-fg-2">
            {rounds === null
              ? 'POV completa con HUD nativo, comms y overlays automáticos.'
              : `${rounds} rondas de POV con HUD nativo, comms y overlays automáticos.`}
          </p>
          <Button asChild variant="hero" size="sm" className="neon-notch h-9 self-start">
            <Link href={produceHref(row.match.id, PRODUCE_FORMAT.full)}>Grabar Full POV</Link>
          </Button>
        </div>
      ) : null}
    </div>
  );
}
