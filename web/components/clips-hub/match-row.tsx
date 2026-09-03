'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ChevronRight, Plus } from 'lucide-react';
import { api } from '@/lib/api';
import {
  fullChipLabel,
  OUTPUT_STATE,
  OUTPUT_TONE,
  pluralClips,
  roundsFromScore,
  shortsChipTone,
  type HubMatch,
} from '@/lib/clips/hub';
import { PRODUCE_FORMAT, produceHref } from '@/lib/clips/routes';
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
  const { match, parsing, shorts, fulls } = row;
  const { ours, theirs } = parseScore(match.score);
  const hasScore = ours !== null && theirs !== null;
  const win = hasScore && ours > theirs;
  const loss = hasScore && ours < theirs;
  const player = match.player ?? '—';
  const expanded = open && !parsing;

  let bar = 'bg-border-strong';
  if (!parsing && win) bar = 'bg-success';
  else if (!parsing && loss) bar = 'bg-destructive';

  return (
    <article
      id={matchRowId(match.id)}
      className={cn('studio-panel studio-enter flex flex-col overflow-hidden rounded-[10px]', expanded && 'studio-panel-raised')}
    >
      <button
        type="button"
        aria-expanded={expanded}
        aria-disabled={parsing || undefined}
        onClick={() => {
          if (!parsing) onToggle();
        }}
        className={cn(
          'flex w-full items-stretch gap-4 px-[18px] py-3 text-left transition-colors duration-(--dur-fast)',
          parsing ? 'cursor-default' : 'cursor-pointer hover:bg-surface-3',
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
            {[player, matchDateLabel(match), match.decentPlays > 0 ? `${match.decentPlays} highlights` : null]
              .filter(Boolean)
              .join(' · ')}
          </span>
        </span>

        {parsing ? (
          <ParsingBlock player={player} />
        ) : (
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
            <span className="flex items-center gap-2 self-center">
              <StatusTag tone={shortsChipTone(shorts)}>Shorts · {shorts.length}</StatusTag>
              <StatusTag tone={fulls[0] === undefined ? OUTPUT_TONE.queue : OUTPUT_TONE[fulls[0].state]}>
                {fulls[0]?.state === OUTPUT_STATE.rec ? (
                  <span aria-hidden className="neon-pulse size-1.5 shrink-0 rounded-full bg-current shadow-[0_0_6px_currentColor]" />
                ) : null}
                {fullChipLabel(fulls)}
              </StatusTag>
            </span>
          </>
        )}

        <ChevronRight
          aria-hidden
          className={cn(
            'size-4 shrink-0 self-center text-fg-3 transition-transform duration-(--dur-fast)',
            expanded && 'rotate-90',
          )}
        />
      </button>

      {expanded ? (
        <div className="studio-enter grid grid-cols-1 border-t border-border-subtle @[44rem]/content:grid-cols-2">
          <ShortsColumn row={row} onChange={onChange} />
          <FullColumn row={row} onChange={onChange} />
          <div className="col-span-full flex items-center justify-end gap-3 border-t border-border-subtle px-4 py-2">
            <span className="font-mono text-meta uppercase tracking-wider text-fg-4">Quitar partida</span>
            <DeleteMatchButton
              label={prettyMapName(match.map)}
              onConfirm={() => api.deleteMatch(match.id)}
              onDeleted={onChange}
            />
          </div>
        </div>
      ) : null}
    </article>
  );
}

function ParsingBlock({ player }: { player: string }): ReactNode {
  return (
    <span role="status" className="flex w-[280px] shrink-0 flex-col justify-center gap-1.5 self-center">
      <span className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-primary">
        <span aria-hidden className="studio-spinner" />
        Parseando POV de {player}
      </span>
      <span className="studio-bar text-primary">
        <span className="studio-indeterminate" />
      </span>
      <span className="font-mono text-meta uppercase tracking-wider text-fg-3">parseando la demo · highlights al terminar</span>
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
      <ColumnHead label="Shorts" accent trailing={pluralClips(row.shorts.length)} />
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
