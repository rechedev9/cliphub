'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, BookmarkPlus, Copy, Layers, Play } from 'lucide-react';
import type { TeamGrenade } from '@/lib/api/grenades';
import type { LineupThrow } from '@/lib/api/lineups';
import type { PracticeSpot } from '@/lib/api/practice';
import { MIN_RELIABLE_SAMPLE } from '@/lib/api/tactical';
import { FOLLOW_UP_SECONDS, FOLLOW_UP_STAY_SECONDS, factLabel, roundKey, type FollowUp } from '@/lib/analysis/grenade-follow-up';
import { grenadeId, spotReplayRefs } from '@/lib/analysis/grenade-patterns';
import { canPractice, NO_ORIGIN_REASON, practiceSpotFor, representativeThrow } from '@/lib/analysis/practice-spots';
import { replayHref, superposedReplayHref } from '@/lib/analysis/replay-link';
import { percentLabel } from '@/lib/library/partidas';
import { lineupThrow, sampleSetpos } from '@/lib/libreta/lineups';
import { roundClock } from '@/lib/replay/tramo';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { SaveLineupButton } from '@/components/libreta/save-lineup-dialog';
import type { LabelledSpot } from './grenade-spot-list';
import { PracticeButton } from './practice-launch';

/** How many follow-up facts the panel lists. */
const SHOWN_FACTS = 8;

type Props = {
  spot: LabelledSpot;
  followUp: FollowUp;
  /** "8 de 31 rondas en T": the spot's rounds over the scope's. */
  denominator: string;
  place: (token: string) => string;
  matchLabel: (matchId: string) => string;
  busy: boolean;
  onPractice: (spots: PracticeSpot[]) => void;
  onSaveLineup: (from: LineupThrow, label: string) => void;
  /** Null where the grenades belong to no team of the library. */
  onSavePlay: (() => void) | null;
  onBack: () => void;
};

/** "Copiar setpos": the console line of a throw; it only copies, CS2 opens with Practicar. */
function CopySetpos({ grenade }: { grenade: TeamGrenade | null }): ReactNode {
  const [copied, setCopied] = useState(false);
  const command =
    grenade === null || grenade.origin === null || grenade.view === null
      ? null
      : sampleSetpos({ x: grenade.origin[0], y: grenade.origin[1], z: grenade.origin[2], yaw: grenade.view[0], pitch: grenade.view[1] });
  return (
    <span title={command ?? NO_ORIGIN_REASON}>
      <Button
        type="button"
        variant="ghost"
        size="xs"
        disabled={command === null}
        onClick={() => {
          if (command === null) return;
          void navigator.clipboard
            .writeText(command)
            .then(() => setCopied(true))
            .catch(() => setCopied(false));
        }}
      >
        <Copy aria-hidden />
        {copied ? 'Setpos copiado' : 'Copiar setpos'}
      </Button>
    </span>
  );
}

/** One grenade read in full: how many of the scope's rounds, what the team does after it, and its rounds. */
export function GrenadeSpotPanel({ spot: labelled, followUp, denominator, place, matchLabel, busy, onPractice, onSaveLineup, onSavePlay, onBack }: Props): ReactNode {
  const { spot, label } = labelled;
  const refs = spotReplayRefs(spot.grenades);
  const best = representativeThrow(spot);
  const bestThrow = best === null ? null : lineupThrow(best);
  const firstOfRound = new Map<string, { grenade: TeamGrenade; count: number }>();
  for (const grenade of spot.grenades) {
    const key = roundKey(grenade.match_id, grenade.round);
    const known = firstOfRound.get(key);
    if (known === undefined) firstOfRound.set(key, { grenade, count: 1 });
    else known.count += 1;
  }

  return (
    <section aria-labelledby="granada-elegida" className="flex flex-col gap-3">
      <Button type="button" variant="ghost" size="xs" className="-ml-2 self-start" onClick={onBack}>
        <ArrowLeft aria-hidden />
        Todas las posiciones
      </Button>
      <header className="flex flex-col gap-1">
        <h2 id="granada-elegida" className="text-body font-semibold text-fg-1">
          {label}
        </h2>
        <p className="text-body-sm text-fg-2">
          <span className="font-semibold text-fg-1 tabular-nums">{denominator}</span>
          {' · '}gana {percentLabel(followUp.won, followUp.rounds)}
          {' · '}
          {spot.throwers.map((thrower) => (thrower.count > 1 ? `${thrower.name} ×${thrower.count}` : thrower.name)).join(', ')}
        </p>
      </header>
      <div className="-ml-2 flex flex-wrap items-center gap-1">
        <Button asChild variant="ghost" size="xs">
          <Link href={superposedReplayHref(refs, label)}>
            <Layers aria-hidden />
            Reproducir {refs.length} {refs.length === 1 ? 'ronda' : 'rondas'}
          </Link>
        </Button>
        <CopySetpos grenade={best} />
        <PracticeButton
          busy={busy}
          disabledReason={best === null ? NO_ORIGIN_REASON : undefined}
          onClick={() => {
            const practice = best === null ? null : practiceSpotFor(best, label);
            if (practice !== null) onPractice([practice]);
          }}
        />
        <SaveLineupButton disabledReason={bestThrow === null ? NO_ORIGIN_REASON : undefined} onClick={() => bestThrow !== null && onSaveLineup(bestThrow, label)} />
        {onSavePlay === null ? null : (
          <Button type="button" variant="ghost" size="xs" onClick={onSavePlay}>
            <BookmarkPlus aria-hidden />
            Guardar como jugada
          </Button>
        )}
      </div>

      <section aria-labelledby="granada-despues" className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <h3 id="granada-despues" className="text-body-sm font-semibold text-fg-1">
            Qué hacen después
          </h3>
          {followUp.rounds < MIN_RELIABLE_SAMPLE ? <Badge variant="outline">Poca muestra</Badge> : null}
        </div>
        {followUp.facts.length === 0 ? (
          <p className="text-body-sm text-fg-2">Sin posiciones en estas rondas: son de demos analizadas con una versión anterior.</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {followUp.facts.slice(0, SHOWN_FACTS).map((fact) => (
              <li key={`${fact.kind}|${fact.value}`} className="flex flex-col gap-0.5">
                <div className="flex items-baseline justify-between gap-2 text-body-sm">
                  <span className="min-w-0 truncate text-fg-1">{factLabel(fact, place)}</span>
                  <span className="shrink-0 text-meta text-fg-2 tabular-nums">
                    {fact.rounds} de {followUp.rounds} · gana {percentLabel(fact.won, fact.rounds)}
                  </span>
                </div>
                <span aria-hidden className="h-1 rounded-full bg-surface-3">
                  <span className="block h-full rounded-full bg-fg-3" style={{ width: `${(fact.rounds / Math.max(1, followUp.rounds)) * 100}%` }} />
                </span>
              </li>
            ))}
          </ul>
        )}
        <p className="text-meta text-fg-3">
          En los {FOLLOW_UP_SECONDS} s después de que explota: qué ronda juegan, dónde es su primer duelo y dónde se queda alguien al menos{' '}
          {FOLLOW_UP_STAY_SECONDS} s. Cada ronda cuenta una vez.
        </p>
      </section>

      <section aria-labelledby="granada-rondas" className="flex flex-col gap-1">
        <h3 id="granada-rondas" className="text-body-sm font-semibold text-fg-1">
          Rondas
        </h3>
        <ul className="flex flex-col divide-y divide-border-subtle">
          {[...firstOfRound.values()].map(({ grenade, count }) => {
            const title = `${matchLabel(grenade.match_id)} · ronda ${grenade.round}`;
            const from = lineupThrow(grenade);
            return (
              <li key={grenadeId(grenade)} className="flex flex-wrap items-center gap-x-3 gap-y-1 py-1 text-meta text-fg-2">
                <span className="min-w-0 flex-1">
                  {title} · {grenade.thrower_name} · <span className="tabular-nums">{roundClock(grenade.seconds)}</span>
                  {count > 1 ? ` · ×${count}` : null}
                  {' · '}
                  <span className={grenade.round_won ? 'text-outcome-win' : 'text-outcome-loss'}>{grenade.round_won ? 'Ganada' : 'Perdida'}</span>
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
                <SaveLineupButton
                  disabledReason={from === null ? NO_ORIGIN_REASON : undefined}
                  onClick={() => from !== null && onSaveLineup(from, `${label} (${grenade.thrower_name})`)}
                />
              </li>
            );
          })}
        </ul>
      </section>
    </section>
  );
}
