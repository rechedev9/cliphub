'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ChevronRight, Layers } from 'lucide-react';
import type { PracticeSpot } from '@/lib/api/practice';
import { spotReplayRefs, type GrenadeSpot } from '@/lib/analysis/grenade-patterns';
import { NO_ORIGIN_REASON, practiceSpotFor, representativeThrow } from '@/lib/analysis/practice-spots';
import { grenadeIcon } from '@/lib/analysis/icons';
import { superposedReplayHref } from '@/lib/analysis/replay-link';
import { percentLabel } from '@/lib/library/partidas';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { cn } from '@/lib/utils';
import { GRENADE_KIND_TOKEN } from './grenade-radar';
import { PracticeButton } from './practice-launch';

export type LabelledSpot = { spot: GrenadeSpot; label: string };

type Props = {
  spots: readonly LabelledSpot[];
  /** The scope's rounds, the denominator of each spot's rounds. */
  total: number;
  highlight: string | null;
  onHover: (key: string | null) => void;
  chosen: ReadonlySet<string>;
  onChoose: (key: string) => void;
  /** True when no more spots fit in one practice cfg. */
  full: boolean;
  busy: boolean;
  onPractice: (spots: PracticeSpot[]) => void;
  onOpen: (key: string) => void;
};

const PAGE = 40;

function throwersText(spot: GrenadeSpot): string {
  return spot.throwers.map((thrower) => (thrower.count > 1 ? `${thrower.name} ×${thrower.count}` : thrower.name)).join(', ');
}

/** The grenade's own silhouette in its kind's colour, the same colour as its tracer on the radar. */
function Swatch({ spot }: { spot: GrenadeSpot }): ReactNode {
  return (
    <span aria-hidden className="mt-0.5 flex w-4 shrink-0 justify-center" style={{ color: `var(${GRENADE_KIND_TOKEN[spot.kind]})` }}>
      <EquipmentIcon name={grenadeIcon(spot.kind)} height={16} />
    </span>
  );
}

/** The matching throws grouped by spot, most rounds first; a click reads one in full. */
export function GrenadeSpotList({ spots, total, highlight, onHover, chosen, onChoose, full, busy, onPractice, onOpen }: Props): ReactNode {
  const [shown, setShown] = useState(PAGE);

  return (
    <div className="flex flex-col">
      <ul className="flex flex-col divide-y divide-border-subtle" onPointerLeave={() => onHover(null)}>
        {spots.slice(0, shown).map(({ spot, label }) => {
          const refs = spotReplayRefs(spot.grenades);
          const best = representativeThrow(spot);
          const isChosen = chosen.has(spot.key);
          return (
            <li
              key={spot.key}
              onPointerEnter={() => onHover(spot.key)}
              className={cn('flex flex-col gap-1 px-2 py-1.5', highlight === spot.key && 'bg-surface-3')}
            >
              <div className="flex items-start gap-2">
                <input
                  type="checkbox"
                  aria-label={`Elegir ${label} para practicar o crear una táctica`}
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
                    onClick={() => onOpen(spot.key)}
                    className="flex items-center gap-1 text-left text-body-sm font-medium text-fg-1 outline-none hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  >
                    <span className="truncate">{label}</span>
                    <ChevronRight aria-hidden className="size-3.5 shrink-0 text-fg-3" />
                  </button>
                  <p className="text-meta text-fg-2">
                    <span className="font-medium text-fg-1 tabular-nums">
                      {spot.rounds} de {total}
                    </span>{' '}
                    rondas · {spot.grenades.length} {spot.grenades.length === 1 ? 'lanzamiento' : 'lanzamientos'}
                    {' · '}
                    gana {percentLabel(spot.roundsWon, spot.rounds)}
                    {' · '}
                    <span className="text-fg-3">{throwersText(spot)}</span>
                  </p>
                  <div className="-ml-2 flex flex-wrap items-center gap-1">
                    <Button asChild variant="ghost" size="xs">
                      <Link href={superposedReplayHref(refs, label)}>
                        <Layers aria-hidden />
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
                </div>
              </div>
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
