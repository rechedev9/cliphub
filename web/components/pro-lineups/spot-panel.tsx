'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { ChevronLeft, ChevronRight, Crosshair, Play, Search } from 'lucide-react';
import {
  lineupHref,
  pct,
  practiceSpot,
  PRO_KIND_LABEL,
  roundClock,
  techniqueLabel,
  type ProLineup,
} from '@/lib/api/pro-lineups';
import { grenadeIcon } from '@/lib/analysis/icons';
import { plainTitle, spotLabel, type ProSpot } from '@/lib/pro-lineups-view';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { SetposCopy } from './setpos-copy';
import { ClipVideo } from './throw-clip';

const ROW = 'flex w-full min-h-11 items-start gap-3 px-3 py-2.5 text-left transition-[background-color] hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-none';

function KindIcon({ kind, size = 16 }: { kind: ProLineup['kind']; size?: number }): ReactNode {
  return (
    <span aria-hidden className="mt-0.5 flex w-4 shrink-0 justify-center" style={{ color: `var(${GRENADE_KIND_TOKEN[kind]})` }}>
      <EquipmentIcon name={grenadeIcon(kind)} height={size} />
    </span>
  );
}

function sidesOf(lineups: readonly ProLineup[]): string {
  const t = lineups.some((l) => l.side === 'T');
  const ct = lineups.some((l) => l.side === 'CT');
  if (t && ct) return 'T y CT';
  return t ? 'T' : 'CT';
}

/** The number a throw point carries on the radar, so a row and its point read as one. */
export function ThrowNumber({ side, number }: { side: ProLineup['side']; number: number }): ReactNode {
  return (
    <span
      aria-hidden
      className={cn(
        'flex size-6 shrink-0 items-center justify-center rounded-full text-meta font-semibold text-surface-0 tabular-nums',
        side === 'CT' ? 'bg-side-ct' : 'bg-side-t',
      )}
    >
      {number}
    </span>
  );
}

function BackButton({ onClick, children }: { onClick: () => void; children: ReactNode }): ReactNode {
  return (
    <Button type="button" variant="ghost" size="xs" className="-ml-2 self-start" onClick={onClick}>
      <ChevronLeft aria-hidden />
      {children}
    </Button>
  );
}

/** Every spot the filters keep, most thrown first; picking one opens it. */
export function SpotList({
  spots,
  mixedKinds,
  text,
  onText,
  hover,
  onHover,
  onOpen,
  onClear,
  video,
}: {
  spots: readonly ProSpot[];
  /** Every row has a clip, so the clip mark would only repeat. */
  video: boolean;
  /** All grenades are shown, so each row names its grenade. */
  mixedKinds: boolean;
  text: string;
  onText: (text: string) => void;
  hover: string | null;
  onHover: (id: string | null) => void;
  onOpen: (spot: ProSpot) => void;
  onClear: (() => void) | null;
}): ReactNode {
  const labelOf = (spot: ProSpot): string => (mixedKinds ? spotLabel(spot) : spot.place || 'Zona sin nombre');
  const seen = new Map<string, number>();
  for (const spot of spots) seen.set(labelOf(spot), (seen.get(labelOf(spot)) ?? 0) + 1);
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-body font-semibold text-fg-1 tabular-nums">
          {spots.length} {spots.length === 1 ? 'destino' : 'destinos'}
        </h2>
        <span className="text-meta text-fg-3">Más tirados primero</span>
      </div>
      <label className="relative">
        <span className="sr-only">Buscar por zona</span>
        <Search aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-fg-3" />
        <Input value={text} onChange={(event) => onText(event.target.value)} placeholder="Window, Mid, CT Spawn" className="pl-8" />
      </label>
      {spots.length === 0 ? (
        <div className="flex flex-col items-start gap-2 rounded-lg border border-border bg-surface-1 px-3 py-3">
          <p className="text-body-sm text-fg-2">Ningún lineup con estos filtros.</p>
          {onClear === null ? null : (
            <Button type="button" variant="outline" size="xs" onClick={onClear}>
              Quitar filtros
            </Button>
          )}
        </div>
      ) : (
        <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
          {spots.map((spot) => (
            <li key={spot.id} onPointerEnter={() => onHover(spot.id)} onPointerLeave={() => onHover(null)}>
              <button
                type="button"
                className={cn(ROW, hover === spot.id && 'bg-surface-2')}
                onClick={() => onOpen(spot)}
                onFocus={() => onHover(spot.id)}
                onBlur={() => onHover(null)}
              >
                <KindIcon kind={spot.kind} />
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-body-sm font-medium text-fg-1">{labelOf(spot)}</span>
                  <span className="text-meta text-fg-2 tabular-nums">
                    {sidesOf(spot.lineups)} · {spot.lineups.length} {spot.lineups.length === 1 ? 'lineup' : 'lineups'} · {spot.throws} lanzamientos
                    {/* Two spots at one callout read apart by where they are mostly thrown from. */}
                    {(seen.get(labelOf(spot)) ?? 0) > 1 ? ` · sobre todo desde ${spot.lineups[0].throw_place || 'zona sin nombre'}` : ''}
                  </span>
                </span>
                {!video && spot.lineups.some((l) => l.clip) ? <Play aria-label="Con vídeo" className="mt-1 size-3.5 shrink-0 text-fg-2" /> : null}
                <ChevronRight aria-hidden className="mt-1 size-4 shrink-0 text-fg-3" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** One spot: every place it is thrown from, with how much of the spot's use each takes. */
export function SpotLineups({
  spot,
  hover,
  onHover,
  onOpen,
  onBack,
  video,
}: {
  spot: ProSpot;
  video: boolean;
  hover: string | null;
  onHover: (id: string | null) => void;
  onOpen: (lineup: ProLineup) => void;
  onBack: () => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      <BackButton onClick={onBack}>Destinos</BackButton>
      <div>
        <h2 className="text-body font-semibold text-fg-1">{spotLabel(spot)}</h2>
        <p className="text-meta text-fg-2 tabular-nums">
          {spot.lineups.length} {spot.lineups.length === 1 ? 'forma de lanzarlo' : 'formas de lanzarlo'} · {spot.throws} lanzamientos pro
        </p>
      </div>
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
        {spot.lineups.map((lineup, index) => {
          const share = spot.throws === 0 ? 0 : lineup.throws.recent / spot.throws;
          return (
            <li key={lineup.id} onPointerEnter={() => onHover(lineup.id)} onPointerLeave={() => onHover(null)}>
              <button
                type="button"
                className={cn(ROW, 'flex-col gap-1.5', hover === lineup.id && 'bg-surface-2')}
                onClick={() => onOpen(lineup)}
                onFocus={() => onHover(lineup.id)}
                onBlur={() => onHover(null)}
              >
                <span className="flex w-full items-start gap-2">
                  <ThrowNumber side={lineup.side} number={index + 1} />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="text-body-sm font-medium text-fg-1">
                      Desde {lineup.throw_place || 'zona sin nombre'}
                      <span className="font-normal text-fg-2"> · {techniqueLabel(lineup)}</span>
                    </span>
                    <span className="text-meta text-fg-2 tabular-nums">
                      {lineup.side} · {lineup.throws.recent} lanzamientos · sale a {roundClock(lineup.clock.median)}
                      {lineup.insta ? ' · insta' : ''}
                    </span>
                  </span>
                  {!video && lineup.clip ? <Play aria-label="Con vídeo" className="mt-1 size-3.5 shrink-0 text-fg-2" /> : null}
                </span>
                <span className="flex w-full items-center gap-2 pl-8">
                  <span aria-hidden className="h-1 flex-1 overflow-hidden rounded-full bg-surface-3">
                    <span className="block h-full rounded-full bg-fg-3" style={{ width: `${Math.max(2, share * 100)}%` }} />
                  </span>
                  <span className="w-10 text-right text-meta text-fg-2 tabular-nums">{pct(share)}</span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/** A lineup inside its spot: the clip, how to throw it and the way to practise it, without leaving the map. */
export function LineupPreview({
  map,
  spot,
  lineup,
  onBack,
  onStep,
}: {
  map: string;
  spot: ProSpot;
  lineup: ProLineup;
  onBack: () => void;
  onStep: (lineup: ProLineup) => void;
}): ReactNode {
  const practice = usePracticeLaunch(map);
  const index = spot.lineups.findIndex((l) => l.id === lineup.id);
  const previous = index > 0 ? spot.lineups[index - 1] : undefined;
  const next = index >= 0 && index < spot.lineups.length - 1 ? spot.lineups[index + 1] : undefined;
  const accuracy = lineup.attempts === 0 ? null : lineup.landed / lineup.attempts;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <BackButton onClick={onBack}>{spotLabel(spot)}</BackButton>
        {spot.lineups.length > 1 ? (
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon-xs" aria-label="Lineup anterior" disabled={previous === undefined} onClick={() => previous && onStep(previous)}>
              <ChevronLeft aria-hidden />
            </Button>
            <span className="text-meta text-fg-2 tabular-nums">
              {index + 1} de {spot.lineups.length}
            </span>
            <Button type="button" variant="ghost" size="icon-xs" aria-label="Lineup siguiente" disabled={next === undefined} onClick={() => next && onStep(next)}>
              <ChevronRight aria-hidden />
            </Button>
          </div>
        ) : null}
      </div>
      {lineup.clip ? <ClipVideo clip={lineup.clip} label={`Lanzamiento de ${plainTitle(lineup.title)}`} /> : null}
      <div className="flex items-start gap-2">
        <ThrowNumber side={lineup.side} number={index + 1} />
        <div className="min-w-0">
          <h2 className="text-body font-semibold text-fg-1">
            {PRO_KIND_LABEL[lineup.kind]} desde {lineup.throw_place || 'zona sin nombre'}
          </h2>
          <p className="text-meta text-fg-2">
            {techniqueLabel(lineup)} · {lineup.side} · {lineup.air_time.toFixed(1)} s en el aire
            {lineup.clip ? '' : ' · aún sin vídeo'}
          </p>
        </div>
      </div>
      <dl className="grid grid-cols-3 gap-2">
        <div className="flex flex-col gap-0.5">
          <dt className="text-meta text-fg-2">Lanzamientos</dt>
          <dd className="text-body-sm font-medium text-fg-1 tabular-nums">{lineup.throws.recent}</dd>
        </div>
        <div className="flex flex-col gap-0.5">
          <dt className="text-meta text-fg-2">Cae donde debe</dt>
          <dd className="text-body-sm font-medium text-fg-1 tabular-nums">{accuracy === null ? 'Sin datos' : pct(accuracy)}</dd>
        </div>
        <div className="flex flex-col gap-0.5">
          <dt className="text-meta text-fg-2">Rondas ganadas</dt>
          <dd className="text-body-sm font-medium text-fg-1 tabular-nums">{pct(lineup.round_win)}</dd>
        </div>
      </dl>
      <SetposCopy lineup={lineup} />
      <div className="flex flex-wrap gap-2">
        <Button type="button" size="sm" disabled={practice.state.kind === 'busy'} onClick={() => practice.launch([practiceSpot(lineup)])}>
          <Crosshair aria-hidden />
          Practicar en CS2
        </Button>
        <Button asChild variant="outline" size="sm">
          <Link href={lineupHref(map, lineup)}>Ver ficha completa</Link>
        </Button>
      </div>
      <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
    </div>
  );
}
