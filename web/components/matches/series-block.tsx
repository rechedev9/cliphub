import type { ReactNode } from 'react';
import Link from 'next/link';
import { Play } from 'lucide-react';
import { prettyMapName } from '@/lib/format';
import { slashDayLabel } from '@/lib/library/day-label';
import { cn } from '@/lib/utils';
import { MapThumb } from '@/components/analysis/map-thumb';
import { TeamLogo } from '@/components/matches/team-logo';
import { Button } from '@/components/ui/button';

/** The EDGE series block, shared by the library series and the public ones: a header strip, then one row per map. */

export type StripBan = { readonly map: string; readonly team: string };

function scoreTone(own: number, other: number): string {
  if (own === other) return 'text-fg-1';
  return own > other ? 'text-success' : 'text-destructive';
}

function bansByTeam(bans: readonly StripBan[]): { team: string; maps: string }[] {
  const groups = new Map<string, string[]>();
  for (const ban of bans) groups.set(ban.team, [...(groups.get(ban.team) ?? []), prettyMapName(ban.map)]);
  return [...groups].map(([team, maps]) => ({ team, maps: maps.join(', ') }));
}

/** Format, both teams with their logos and the series score, the event, at the right the bans of each team, then `actions`. */
export function SeriesStrip({
  format,
  title,
  teams,
  score,
  event,
  bans,
  actions,
}: {
  format: string | null;
  title: string;
  /** The two team names, or null when the series does not name both. */
  teams: readonly [string, string] | null;
  score: readonly [number, number] | null;
  event?: string;
  bans: readonly StripBan[];
  /** Controls at the far right, such as the block's kebab menu. */
  actions?: ReactNode;
}): ReactNode {
  const banGroups = bansByTeam(bans);
  return (
    <div role="group" aria-label={title} className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-border bg-surface-3 px-3 py-2 text-body-sm">
      {format ? <span className="font-bold tracking-wider text-fg-1">{format}</span> : null}
      {teams ? (
        <span className="flex min-w-0 items-center gap-2 font-semibold text-fg-1">
          <TeamLogo name={teams[0]} />
          <span className="truncate">{teams[0]}</span>
          {score ? (
            <span className="font-bold tabular-nums">
              <span className={scoreTone(score[0], score[1])}>{score[0]}</span>
              <span aria-hidden className="px-1.5 text-fg-3">:</span>
              <span className="sr-only"> a </span>
              <span className={scoreTone(score[1], score[0])}>{score[1]}</span>
            </span>
          ) : (
            <span aria-hidden className="text-fg-3">vs</span>
          )}
          <span className="truncate">{teams[1]}</span>
          <TeamLogo name={teams[1]} />
        </span>
      ) : (
        <span className="truncate font-semibold text-fg-1">{title}</span>
      )}
      {event ? <span className="min-w-0 truncate text-meta text-fg-3">{event}</span> : null}
      {banGroups.length > 0 ? (
        <span className="ml-auto flex flex-wrap items-center gap-x-3 text-meta text-fg-3">
          <span className="font-medium tracking-wider">BANS</span>
          {banGroups.map(({ team, maps }) => (
            <span key={team}>
              {team === '' ? null : <span className="font-semibold text-fg-2">{team} </span>}
              {maps}
            </span>
          ))}
        </span>
      ) : null}
      {actions ? <span className={cn('flex shrink-0 items-center gap-1', banGroups.length === 0 && 'ml-auto')}>{actions}</span> : null}
    </div>
  );
}

const EDGE_CLASS = {
  win: 'border-l-success',
  loss: 'border-l-destructive',
  none: 'border-l-border-strong',
} as const;

export type MapEdge = keyof typeof EDGE_CLASS;

export type MapTag = { readonly text: 'PICK' | 'DECIDER'; readonly title: string };

/** "PICK de FURIA", "PICK" or "DECIDER" as the tag over a map thumbnail; the full text stays on its title. */
export function mapTag(label: string): MapTag {
  return { text: label.startsWith('DECIDER') ? 'DECIDER' : 'PICK', title: label };
}

/** The map's picture with its day or its PICK tag on top, the bold name below and a coloured left edge. */
export function MapCell({ map, day, name, tag, edge = 'none' }: { map: string | null; day: string; name: ReactNode; tag?: MapTag; edge?: MapEdge }): ReactNode {
  return (
    <div className={cn('relative h-16 w-44 shrink-0 overflow-hidden border-l-[3px] bg-surface-0', EDGE_CLASS[edge])}>
      {map === null ? null : <MapThumb map={map} photo className="absolute inset-0 size-full rounded-none" crop={1.5} />}
      <div className="absolute inset-0 bg-linear-to-r from-black/75 via-black/35 to-black/5" />
      <div className="relative flex h-full flex-col justify-between px-2 py-1.5 text-white">
        {tag ? (
          <span title={tag.title} className="text-meta leading-none font-semibold tracking-wider">
            {tag.text}
            <span className="sr-only">{tag.title === tag.text ? '' : `: ${tag.title}`}</span>
          </span>
        ) : (
          <span className="text-meta leading-none tracking-wide text-white/80">{day}</span>
        )}
        <span className="truncate text-body leading-tight font-bold">{name}</span>
      </div>
    </div>
  );
}

/** The green 2D button in a cell of its own, set off by a divider. */
export function TwoDCell({ children }: { children: ReactNode }): ReactNode {
  return <div className="flex shrink-0 items-center border-l border-border-subtle px-3">{children}</div>;
}

export function TwoDLink({ href, label }: { href: string; label: string }): ReactNode {
  return (
    <Button asChild variant="default" size="xs">
      <Link href={href} aria-label={label}>
        <Play aria-hidden />
        2D
      </Link>
    </Button>
  );
}

/** A date group heading in the slash style: an orange slash, then the day in bold. */
export function DayHeading({ day }: { day: string }): ReactNode {
  const label = slashDayLabel(day);
  return (
    <h2 aria-label={label} className="flex items-center gap-2 text-body-sm font-bold text-fg-1">
      <span aria-hidden className="text-slash">/</span>
      <span aria-hidden>{label}</span>
    </h2>
  );
}
