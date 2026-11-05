'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ChevronDown, ChevronRight, Play } from 'lucide-react';
import type { TeamGrenade } from '@/lib/api/grenades';
import type { PracticeSpot } from '@/lib/api/practice';
import { grenadeId, spotReplayRefs, type GrenadeSpot } from '@/lib/analysis/grenade-patterns';
import { canPractice, NO_ORIGIN_REASON, practiceSpotFor, representativeThrow } from '@/lib/analysis/practice-spots';
import { replayHref } from '@/lib/analysis/replay-link';
import { percentLabel } from '@/lib/library/partidas';
import { elapsedLabel } from '@/lib/tactical-spatial';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { GRENADE_KIND_TOKEN } from './grenade-radar';
import { PracticeButton } from './practice-launch';

export type LabelledSpot = { spot: GrenadeSpot; label: string };

type Props = {
  spots: readonly LabelledSpot[];
  highlight: string | null;
  onHover: (key: string | null) => void;
  chosen: ReadonlySet<string>;
  onChoose: (key: string) => void;
  /** True when no more spots fit in one practice cfg. */
  full: boolean;
  busy: boolean;
  onPractice: (spots: PracticeSpot[]) => void;
  matchLabel: (matchId: string) => string;
};

const PAGE = 40;

function throwersText(spot: GrenadeSpot): string {
  return spot.throwers.map((thrower) => (thrower.count > 1 ? `${thrower.name} ×${thrower.count}` : thrower.name)).join(', ');
}

function Swatch({ spot }: { spot: GrenadeSpot }): ReactNode {
  return (
    <span
      aria-hidden
      className="mt-1.5 size-2.5 shrink-0 rounded-full"
      style={{ backgroundColor: `var(${GRENADE_KIND_TOKEN[spot.kind]})` }}
    />
  );
}

/** The matching throws grouped by spot, most repeated first, each with its replay and its practice. */
export function GrenadeSpotList({ spots, highlight, onHover, chosen, onChoose, full, busy, onPractice, matchLabel }: Props): ReactNode {
  const [open, setOpen] = useState<string | null>(null);
  const [shown, setShown] = useState(PAGE);

  return (
    <div className="flex flex-col">
      <ul className="flex flex-col divide-y divide-border-subtle" onPointerLeave={() => onHover(null)}>
        {spots.slice(0, shown).map(({ spot, label }) => {
          const refs = spotReplayRefs(spot.grenades);
          const best = representativeThrow(spot);
          const isOpen = open === spot.key;
          const isChosen = chosen.has(spot.key);
          return (
            <li
              key={spot.key}
              onPointerEnter={() => onHover(spot.key)}
              className={cn('flex flex-col gap-2 px-2 py-2.5', highlight === spot.key && 'bg-surface-3')}
            >
              <div className="flex items-start gap-2">
                <input
                  type="checkbox"
                  aria-label={`Elegir ${label} para practicar`}
                  title={best === null ? NO_ORIGIN_REASON : undefined}
                  checked={isChosen}
                  disabled={best === null || (full && !isChosen)}
                  onChange={() => onChoose(spot.key)}
                  className="mt-1 size-4 accent-primary"
                />
                <Swatch spot={spot} />
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <button
                    type="button"
                    aria-expanded={isOpen}
                    onClick={() => setOpen(isOpen ? null : spot.key)}
                    className="flex items-center gap-1 text-left text-body-sm font-medium text-fg-1 outline-none hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  >
                    {isOpen ? <ChevronDown aria-hidden className="size-3.5 shrink-0" /> : <ChevronRight aria-hidden className="size-3.5 shrink-0" />}
                    <span className="truncate">{label}</span>
                  </button>
                  <p className="text-meta text-fg-2">
                    <span className="tabular-nums">{spot.grenades.length}</span> {spot.grenades.length === 1 ? 'lanzamiento' : 'lanzamientos'}
                    {' · '}
                    {percentLabel(spot.roundsWon, spot.rounds)} rondas ganadas
                    {' · '}
                    <span className="text-fg-3">{throwersText(spot)}</span>
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-1.5 pl-11">
                <Button asChild variant="ghost" size="xs">
                  <Link href={replayHref(refs, label)}>
                    <Play aria-hidden />
                    Reproducir {refs.length} {refs.length === 1 ? 'ronda' : 'rondas'}
                  </Link>
                </Button>
                <PracticeButton
                  busy={busy}
                  disabledReason={best === null ? NO_ORIGIN_REASON : undefined}
                  onClick={() => {
                    const practice = best === null ? null : practiceSpotFor(best, label);
                    if (practice !== null) onPractice([practice]);
                  }}
                />
              </div>
              {isOpen ? <ThrowList grenades={spot.grenades} label={label} busy={busy} onPractice={onPractice} matchLabel={matchLabel} /> : null}
            </li>
          );
        })}
      </ul>
      {spots.length > shown ? (
        <Button type="button" variant="ghost" size="sm" className="self-start" onClick={() => setShown((count) => count + PAGE)}>
          Mostrar {Math.min(PAGE, spots.length - shown)} más
        </Button>
      ) : null}
    </div>
  );
}

function ThrowList({
  grenades,
  label,
  busy,
  onPractice,
  matchLabel,
}: {
  grenades: readonly TeamGrenade[];
  label: string;
  busy: boolean;
  onPractice: (spots: PracticeSpot[]) => void;
  matchLabel: (matchId: string) => string;
}): ReactNode {
  return (
    <ul className="ml-11 flex flex-col gap-1 border-l border-border-subtle pl-3">
      {grenades.map((grenade) => {
        const title = `${matchLabel(grenade.match_id)} · ronda ${grenade.round}`;
        return (
          <li key={grenadeId(grenade)} className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-2">
            <span className="min-w-0 flex-1">
              {title} · {grenade.thrower_name} · <span className="tabular-nums">{elapsedLabel(grenade.seconds)}</span>
              {' · '}
              <span className={grenade.round_won ? 'text-outcome-win' : 'text-outcome-loss'}>
                {grenade.round_won ? 'Ganada' : 'Perdida'}
              </span>
            </span>
            <Button asChild variant="ghost" size="xs">
              <Link href={replayHref(spotReplayRefs([grenade]), title)}>
                <Play aria-hidden />
                Reproducir
              </Link>
            </Button>
            <PracticeButton
              busy={busy}
              label="Practicar"
              disabledReason={canPractice(grenade) ? undefined : NO_ORIGIN_REASON}
              onClick={() => {
                const practice = practiceSpotFor(grenade, `${label} (${grenade.thrower_name})`);
                if (practice !== null) onPractice([practice]);
              }}
            />
          </li>
        );
      })}
    </ul>
  );
}
