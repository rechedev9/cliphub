'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import type { LibraryMatchPlayer } from '@/lib/api/library';
import type { RoundTimeline, WinPoint } from '@/lib/api/metrics';
import { matchHref } from '@/lib/clips/routes';
import { roundEvidenceHref } from '@/lib/library/rounds';
import { percentText, ppText, winChart } from '@/lib/metrics/advanced';
import { secondsText } from '@/lib/reports/scout-sections';
import { RelatedList } from '@/components/records/related-list';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

const WIDTH = 640;

function deltaClass(delta: number): string {
  if (delta > 0) return 'text-side-ct';
  if (delta < 0) return 'text-side-t';
  return 'text-fg-3';
}
const HEIGHT = 160;

function pointText(point: WinPoint, names: ReadonlyMap<number, string>): string {
  const actor = point.actor_slot === undefined ? '' : (names.get(point.actor_slot) ?? `Jugador ${point.actor_slot}`);
  const target = point.target_slot === undefined ? '' : (names.get(point.target_slot) ?? `Jugador ${point.target_slot}`);
  switch (point.kind) {
    case 'start':
      return 'Fin del tiempo de compra';
    case 'end':
      return 'Fin de la ronda';
    case 'kill':
      return actor ? `${actor} mata a ${target}` : `${target} muere`;
    case 'plant':
      return actor ? `${actor} planta la bomba` : 'Bomba plantada';
    case 'defuse':
      return actor ? `${actor} desactiva la bomba` : 'Bomba desactivada';
    case 'explode':
      return 'Explota la bomba';
    default:
      return point.kind;
  }
}

/**
 * A round's CT win probability from freeze end to its end, one step per kill,
 * plant or defuse. Every step opens its tick in the replay.
 */
export function WinProbability({
  matchId,
  timelines,
  players,
}: {
  matchId: string;
  timelines: readonly RoundTimeline[];
  players: readonly LibraryMatchPlayer[];
}): ReactNode {
  const router = useRouter();
  const [round, setRound] = useState(String(timelines[0]?.round ?? ''));
  const timeline = timelines.find((t) => String(t.round) === round) ?? timelines[0];
  if (!timeline) return null;
  const names = new Map(players.map((p) => [p.slot, p.name || p.steamid64]));
  const chart = winChart(timeline, WIDTH, HEIGHT);
  const href = (point: WinPoint): string => roundEvidenceHref(matchHref(matchId), timeline.round, point.tick);
  return (
    <RelatedList
      title="Probabilidad de victoria por ronda"
      count={timelines.length}
      actions={
        <Select value={String(timeline.round)} onValueChange={setRound}>
          <SelectTrigger aria-label="Ronda" className="h-8 min-w-28">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {timelines.map((t) => (
              <SelectItem key={t.round} value={String(t.round)}>
                Ronda {t.round}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      }
    >
      <div className="flex flex-col gap-3 px-4 py-3">
        <p className="text-body-sm text-fg-2">
          Probabilidad de que gane el lado CT. Gana la ronda{' '}
          <span className={cn('font-medium', timeline.winner === 'CT' ? 'text-side-ct' : 'text-side-t')}>{timeline.winner || 'sin dato'}</span>.
        </p>
        <svg
          viewBox={`-8 -8 ${WIDTH + 16} ${HEIGHT + 24}`}
          className="h-auto w-full max-w-3xl"
          role="img"
          aria-label={`Probabilidad de victoria CT en la ronda ${timeline.round}`}
        >
          <line x1={0} x2={WIDTH} y1={HEIGHT / 2} y2={HEIGHT / 2} className="stroke-border" strokeDasharray="4 4" />
          <line x1={0} x2={WIDTH} y1={HEIGHT} y2={HEIGHT} className="stroke-border-strong" />
          <text x={0} y={HEIGHT + 14} className="fill-fg-3 text-[10px]">0:00</text>
          <text x={WIDTH} y={HEIGHT + 14} textAnchor="end" className="fill-fg-3 text-[10px]">
            {secondsText(chart.maxSeconds)}
          </text>
          <text x={WIDTH} y={-1} textAnchor="end" className="fill-side-ct text-[10px]">CT 100 %</text>
          <text x={WIDTH} y={HEIGHT - 3} textAnchor="end" className="fill-side-t text-[10px]">T 100 %</text>
          <path d={chart.path} fill="none" className="stroke-primary" strokeWidth={2} />
          {chart.points.map(({ x, y, point }, i) => (
            <circle
              key={`${point.tick}:${i}`}
              cx={x}
              cy={y}
              r={4}
              className="cursor-pointer fill-surface-1 stroke-primary"
              strokeWidth={2}
              onClick={() => router.push(href(point))}
            >
              <title>{`${secondsText(point.seconds)} ${pointText(point, names)}: CT ${percentText(point.probability_ct)}`}</title>
            </circle>
          ))}
        </svg>
        <ol className="flex flex-col divide-y divide-border-subtle text-body-sm">
          {timeline.points.map((point, i) => {
            const prev = timeline.points[i - 1];
            const delta = prev ? point.probability_ct - prev.probability_ct : 0;
            return (
              <li key={`${point.tick}:${i}`} className="flex flex-wrap items-baseline gap-x-3 py-1.5">
                <span className="w-10 font-mono text-fg-3">{secondsText(point.seconds)}</span>
                <Link href={href(point)} className="min-w-48 flex-1 text-fg-1 hover:text-primary hover:underline">
                  {pointText(point, names)}
                </Link>
                <span className="font-mono text-fg-3">
                  {point.alive_ct}v{point.alive_t}
                </span>
                <span className="w-16 text-right font-mono text-fg-1">CT {percentText(point.probability_ct)}</span>
                <span className={cn('w-20 text-right font-mono', deltaClass(delta))}>
                  {i === 0 ? '' : ppText(delta)}
                </span>
              </li>
            );
          })}
        </ol>
      </div>
    </RelatedList>
  );
}
