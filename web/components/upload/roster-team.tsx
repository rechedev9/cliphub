'use client';

import type { ReactNode } from 'react';
import type { DemoPlayer } from '@/lib/api/types';
import { formatDecimal, ratingBarPct } from '@/lib/format';
import { cn } from '@/lib/utils';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { SelectionMark } from '@/components/clips/selection-mark';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';

/** Series rows add mapsPresent; single-match callers keep plain DemoPlayer values. */
export type PickerPlayer = DemoPlayer & { mapsPresent?: number };

const TEAM_META = {
  T: { label: 'Terroristas', tint: 'bg-side-t/18', chip: 'bg-side-t text-surface-0', avatar: 'bg-side-t/28', avatarOn: 'bg-side-t text-surface-0' },
  CT: { label: 'Antiterroristas', tint: 'bg-side-ct/18', chip: 'bg-side-ct text-surface-0', avatar: 'bg-side-ct/28', avatarOn: 'bg-side-ct text-surface-0' },
  '': { label: 'Otros', tint: 'bg-surface-3', chip: 'bg-surface-5 text-fg-1', avatar: 'bg-surface-4', avatarOn: 'bg-surface-5' },
} as const;

/** Static border classes, since Tailwind only sees whole class names. */
const SELECTED_BORDER: Record<FormatTone, string> = {
  short: 'border-format-short',
  full: 'border-format-full',
  stream: 'border-format-stream',
  summary: 'border-format-summary',
};

const STAT_TOOLTIPS = {
  kd: 'Kills y muertes',
  a: 'Asistencias',
  pm: 'Diferencia entre kills y muertes',
  adr: 'Daño medio por ronda',
  kast: '% de rondas con kill/asistencia/sobrevivir/trade',
  hs: '% de kills por headshot',
  rating: 'Rating HLTV 1.0',
  mvp: 'Rondas como MVP',
} as const;

// HLTV 1.0 bands: strong, average, below average, poor.
const RATING_BANDS = [
  { min: 1.15, text: 'text-success', bar: 'bg-success' },
  { min: 0.95, text: 'text-fg-1', bar: 'bg-fg-3' },
  { min: 0.8, text: 'text-warning', bar: 'bg-warning' },
  { min: Number.NEGATIVE_INFINITY, text: 'text-destructive', bar: 'bg-destructive' },
] as const;

function ratingBand(rating: number): (typeof RATING_BANDS)[number] {
  return RATING_BANDS.find((band) => rating >= band.min) ?? RATING_BANDS[RATING_BANDS.length - 1];
}

function signed(n: number): string {
  return n > 0 ? `+${n}` : `${n}`;
}

/** First one or two glyphs of a name, uppercased, for the card's monogram avatar. */
function initials(name: string): string {
  return Array.from(name.trim()).slice(0, 2).join('').toUpperCase();
}

type HighlightChip = { key: string; label: string; className: string };

/** Nonzero multi-kill chips in ACE, 4K, 3K order; ACE wears the flow's colour. */
function highlightChips(p: DemoPlayer, tone: FormatTone): HighlightChip[] {
  const chips: HighlightChip[] = [];
  if (p.rounds5k) chips.push({ key: 'ace', label: `ACE ×${p.rounds5k}`, className: FORMAT_TONE[tone].fill });
  if (p.rounds4k) chips.push({ key: '4k', label: `4K ×${p.rounds4k}`, className: cn(FORMAT_TONE[tone].tint, 'text-fg-1') });
  if (p.rounds3k) chips.push({ key: '3k', label: `3K ×${p.rounds3k}`, className: 'bg-surface-4 text-fg-1' });
  return chips;
}

export type RosterTeamProps = {
  side: DemoPlayer['team'];
  roster: PickerPlayer[];
  /** The side's final score, when the scan has one. */
  score: number | null;
  tone: FormatTone;
  chosen: ReadonlySet<string>;
  recommendedId: string | undefined;
  recommendedLabel: string;
  /** Series map count; flags a player missing from some maps. */
  seriesMapCount: number | null;
  /** "Varios del equipo": each card shows a tick mark. */
  multi: boolean;
  /** Any player of the match has an MVP round, so the MVP stat is worth a slot. */
  showMvp: boolean;
  onChoose: (steamId: string) => void;
  /** "Todo el equipo", offered while several players can be picked. */
  onPickTeam?: () => void;
};

/** One side of the scoreboard: a header tinted by the side, then its players as selectable cards. */
export function RosterTeam(props: RosterTeamProps): ReactNode {
  const { side, roster, score, onPickTeam } = props;
  const meta = TEAM_META[side];
  const avg = roster.reduce((s, p) => s + p.rating, 0) / roster.length;
  return (
    <section aria-label={meta.label} className="flex min-w-0 flex-col gap-2">
      <div className={cn('flex flex-wrap items-center gap-x-2.5 gap-y-2 rounded-[14px] px-3.5 py-3', meta.tint)}>
        {side === '' ? null : <span className={cn('inline-flex h-5.5 items-center rounded-md px-1.75 text-meta font-extrabold', meta.chip)}>{side}</span>}
        <span className="min-w-0 flex-1 truncate font-display text-body font-extrabold text-fg-1">{meta.label}</span>
        <span className="text-meta tabular-nums text-fg-3">media {formatDecimal(avg, 2)}</span>
        {onPickTeam ? (
          <Button type="button" variant="outline" size="xs" onClick={onPickTeam}>
            Todo el equipo
          </Button>
        ) : null}
        {score === null ? null : <span className="text-title font-extrabold tabular-nums text-fg-1">{score}</span>}
      </div>
      {roster.map((p) => (
        <PlayerCard key={p.steamId} player={p} team={props} />
      ))}
    </section>
  );
}

function PlayerCard({ player: p, team }: { player: PickerPlayer; team: RosterTeamProps }): ReactNode {
  const { tone, chosen, recommendedId, recommendedLabel, seriesMapCount, onChoose, multi, showMvp } = team;
  const meta = TEAM_META[team.side];
  const active = chosen.has(p.steamId);
  const chips = highlightChips(p, tone);
  const band = ratingBand(p.rating);
  const plusMinus = p.kills - p.deaths;
  const missingMaps = seriesMapCount !== null && typeof p.mapsPresent === 'number' && p.mapsPresent < seriesMapCount;
  const stats: Array<{ key: keyof typeof STAT_TOOLTIPS; text: string; className?: string }> = [
    { key: 'kd', text: `${p.kills}-${p.deaths}` },
    { key: 'a', text: `${p.assists} A` },
    { key: 'pm', text: signed(plusMinus), className: plusMinus >= 0 ? 'text-fg-1' : undefined },
    { key: 'adr', text: `${Math.round(p.adr)} ADR` },
    { key: 'kast', text: `KAST ${Math.round(p.kast)}%` },
    { key: 'hs', text: `HS ${Math.round(p.hsPct)}%` },
    ...(showMvp ? [{ key: 'mvp' as const, text: `${p.mvps} MVP` }] : []),
  ];
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={() => onChoose(p.steamId)}
      className={cn(
        'grid w-full cursor-pointer items-center gap-3 rounded-[14px] border-[1.5px] px-3.5 py-3 text-left',
        multi ? 'grid-cols-[1.5rem_2.75rem_minmax(0,1fr)_5.5rem_auto]' : 'grid-cols-[2.75rem_minmax(0,1fr)_5.5rem_auto]',
        'transition-[background-color,border-color] duration-(--dur-fast) ease-standard',
        FOCUS_RING,
        active ? cn(SELECTED_BORDER[tone], FORMAT_TONE[tone].tint) : 'border-transparent bg-surface-2 hover:bg-surface-3',
      )}
    >
      {multi ? <SelectionMark selected={active} className={cn('rounded-md', active && cn(SELECTED_BORDER[tone], FORMAT_TONE[tone].fill))} /> : null}
      <span
        data-testid="player-avatar"
        aria-hidden
        className={cn('grid size-11 place-items-center rounded-xl font-display text-body-sm font-extrabold', active ? meta.avatarOn : cn(meta.avatar, 'text-fg-1'))}
      >
        {initials(p.name)}
      </span>
      <span className="flex min-w-0 flex-col gap-1">
        <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <span className="min-w-0 truncate text-body font-bold text-fg-1">{p.name}</span>
          {p.steamId === recommendedId ? (
            <span className="inline-flex h-5.5 items-center rounded-md bg-success/22 px-1.75 text-[0.6875rem] font-extrabold text-fg-1">{recommendedLabel}</span>
          ) : null}
          {missingMaps ? <StatusTag className="tabular-nums">{p.mapsPresent}/{seriesMapCount} mapas</StatusTag> : null}
        </span>
        <span className="text-meta tabular-nums text-fg-3">
          {stats.map((stat, index) => (
            <span key={stat.key}>
              {index > 0 ? ' · ' : null}
              <span title={STAT_TOOLTIPS[stat.key]} className={cn('whitespace-nowrap', stat.className)}>{stat.text}</span>
            </span>
          ))}
        </span>
      </span>
      <span className="flex flex-col gap-1.5" title={STAT_TOOLTIPS.rating}>
        <span className="text-meta text-fg-2">
          Rating <b className={cn('font-bold tabular-nums', band.text)}>{formatDecimal(p.rating, 2)}</b>
        </span>
        <span aria-hidden className="h-1.5 rounded-full bg-surface-4">
          <span className={cn('block h-full rounded-full', band.bar)} style={{ width: `${ratingBarPct(p.rating)}%` }} />
        </span>
      </span>
      <span className="flex max-w-24 flex-wrap justify-end gap-1">
        {chips.map((c) => (
          <span key={c.key} className={cn('inline-flex h-5.5 items-center rounded-md px-1.75 text-[0.6875rem] font-extrabold tabular-nums', c.className)}>
            {c.label}
          </span>
        ))}
      </span>
    </button>
  );
}
