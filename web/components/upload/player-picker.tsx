'use client';

import { useEffect, useState, type CSSProperties, type ReactNode } from 'react';
import { Users } from 'lucide-react';
import type { DemoPlayer, RosterMatch } from '@/lib/api/types';
import { cn } from '@/lib/utils';
import { ratingBarPct, prettyMapName } from '@/lib/format';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { StudioDataRow } from '@/components/studio/data-row';
import { Badge } from '@/components/ui/badge';
import { Button, FOCUS_RING } from '@/components/ui/button';

/**
 * A picker row is a scan's DemoPlayer, optionally carrying `mapsPresent` in
 * series mode (how many of the series' maps the player appeared in). It stays
 * optional so single-match callers pass a plain DemoPlayer unchanged.
 */
export type PickerPlayer = DemoPlayer & { mapsPresent?: number };

export type PlayerPickerProps = {
  /** Roster from the scan (single match) or aggregated across a series, sorted by kills desc. */
  players: PickerPlayer[];
  /** Fires when the user confirms the currently selected target. */
  onPick: (steamId: string) => void;
  /** Match-level context (map, score, rounds) shown above the tables, when the scan has it. */
  match?: RosterMatch;
  /**
   * Total maps scanned in a bulk series. When 2+, the picker shows a series
   * summary strip instead of the single-match header and flags players missing
   * from some maps. Absent/1 keeps the plain single-match rendering.
   */
  seriesMapCount?: number;
};

/** Tooltip copy for the abbreviated stat column headers. */
const STAT_TOOLTIPS: Record<string, string> = {
  rating: 'Rating HLTV 1.0',
  adr: 'Daño medio por ronda',
  kast: '% de rondas con kill/asistencia/sobrevivir/trade',
  hs: '% de kills por headshot',
};

/**
 * The HLTV-1.0 performance ramp, in tokens. `lib/format.ts` still maps the same
 * bands onto raw `emerald-400`/`amber-400`/`rose-400`, which neither track the
 * theme nor sit on the navy canvas' hue; the scoreboard keeps a token-mapped
 * copy of the bands until that shared helper is migrated.
 */
const RATING_BANDS = [
  { min: 1.15, text: 'text-success', bar: 'bg-success' },
  { min: 0.95, text: 'text-fg-1', bar: 'bg-fg-2' },
  { min: 0.8, text: 'text-warning', bar: 'bg-warning' },
  { min: Number.NEGATIVE_INFINITY, text: 'text-destructive', bar: 'bg-destructive' },
] as const;

function ratingBand(rating: number): (typeof RATING_BANDS)[number] {
  return RATING_BANDS.find((band) => rating >= band.min) ?? RATING_BANDS[RATING_BANDS.length - 1];
}

/** First one or two glyphs of a name, uppercased, for the row's monogram avatar. */
function initials(name: string): string {
  return Array.from(name.trim()).slice(0, 2).join('').toUpperCase();
}

/**
 * Clip-worthiness score for the "Recommended" pick: multi-kill rounds are the
 * strongest signal a player's POV makes a good reel, weighted by kill count.
 */
function clipScore(p: DemoPlayer): number {
  return 3 * (p.rounds5k ?? 0) + 2 * (p.rounds4k ?? 0) + 1 * (p.rounds3k ?? 0);
}

/** Highest clip score in the roster, tiebroken by rating; undefined for an empty roster. */
function pickRecommended(players: DemoPlayer[]): DemoPlayer | undefined {
  return players.reduce<DemoPlayer | undefined>((best, p) => {
    if (!best) return p;
    const bestScore = clipScore(best);
    const score = clipScore(p);
    return score > bestScore || (score === bestScore && p.rating > best.rating) ? p : best;
  }, undefined);
}

type HighlightChip = { key: string; label: string; tone: StatusTagTone };

/** Nonzero multi-kill chips in ACE -> 4K -> 3K order; ACE gets the strongest (cyan) treatment. */
function highlightChips(p: DemoPlayer): HighlightChip[] {
  const chips: HighlightChip[] = [];
  if (p.rounds5k) chips.push({ key: 'ace', label: `ACE ×${p.rounds5k}`, tone: 'primary' });
  if (p.rounds4k) chips.push({ key: '4k', label: `4K ×${p.rounds4k}`, tone: 'warning' });
  if (p.rounds3k) chips.push({ key: '3k', label: `3K ×${p.rounds3k}`, tone: 'neutral' });
  return chips;
}

/** A scoreboard column: label, the value to render, and an optional colour tone. */
type Column = {
  key: string;
  label: string;
  value: (p: DemoPlayer) => string;
  tone?: (p: DemoPlayer) => string;
  /** Secondary columns hide on a narrow column so the player name never collapses. */
  secondary?: boolean;
  /** Overrides the default text cell, e.g. the rating column's value-plus-bar. */
  render?: (p: DemoPlayer) => ReactNode;
};

/** Rating cell: the number plus a bar showing it against a 2.0 (elite-pace) ceiling. */
function RatingCell({ rating }: { rating: number }) {
  const band = ratingBand(rating);
  return (
    <span className="flex flex-col items-end gap-1">
      <span className={band.text}>{rating.toFixed(2)}</span>
      <span aria-hidden className="h-[3px] w-10 bg-surface-0">
        <span className={cn('block h-full', band.bar)} style={{ width: `${ratingBarPct(rating)}%` }} />
      </span>
    </span>
  );
}

function signed(n: number): string {
  return n > 0 ? `+${n}` : `${n}`;
}

const TEAM_META = {
  T: { label: 'Terroristas', text: 'text-warning', chip: 'border-warning/45 bg-warning/10 text-warning' },
  CT: { label: 'Antiterroristas', text: 'text-primary', chip: 'border-primary/45 bg-primary/10 text-primary' },
  '': { label: 'Otros', text: 'text-fg-2', chip: 'border-border-strong bg-surface-3 text-fg-2' },
} as const;

/** Compact match summary (map, final score, rounds) shown above the roster tables. */
function MatchHeader({ match }: { match: RosterMatch }) {
  const tWon = match.scoreT > match.scoreCt;
  const ctWon = match.scoreCt > match.scoreT;
  return (
    <StudioDataRow
      label={prettyMapName(match.map)}
      value={
        <span
          className="inline-flex items-baseline gap-1.5 text-body-lg"
          aria-label={`Marcador ${match.scoreT} a ${match.scoreCt}`}
        >
          <span className={tWon ? 'font-bold text-warning' : 'text-fg-3'}>{match.scoreT}</span>
          {/* A rule, not a hyphen: a literal separator inside a tabular-nums run
              renders at digit width and jitters against the figures. */}
          <span aria-hidden className="h-4 w-px self-center bg-border-strong" />
          <span className={ctWon ? 'font-bold text-primary' : 'text-fg-3'}>{match.scoreCt}</span>
        </span>
      }
      status={<StatusTag>{match.rounds} rondas</StatusTag>}
    />
  );
}

/**
 * Compact series header shown above the roster in series mode: it replaces the
 * single-match score line (a series has no single map/score) with the map count
 * and roster size, so the user knows the scoreboard is aggregated across maps.
 */
function SeriesSummary({ mapCount, playerCount }: { mapCount: number; playerCount: number }) {
  return (
    <StudioDataRow
      icon={Users}
      active
      label={`Serie · ${mapCount} mapas`}
      status={<StatusTag tone="primary">estadísticas combinadas · {playerCount} jugadores</StatusTag>}
    />
  );
}

/**
 * PlayerPicker — pick whose POV to clip after a roster scan, shown as a CS-style
 * scoreboard split by team (Terroristas / Antiterroristas), with a match header
 * (map, score, rounds) above it when the scan reports one. Each team is its own
 * table with column headers; rows carry HLTV rating, K/D/A, +/-, ADR, KAST, HS%
 * (and MVP when the demo reports it), plus a Highlights line of multi-kill chips
 * under the player name. The roster's clip-worthiest player (by multi-kill rounds,
 * the strongest signal for a good reel) is auto-highlighted and tagged
 * "Recomendado". Clicking a row changes the selection; the explicit continue
 * action confirms it.
 */
export function PlayerPicker({ players, onPick, match, seriesMapCount }: PlayerPickerProps) {
  const recommended = pickRecommended(players);
  const [selected, setSelected] = useState<string | null>(recommended?.steamId ?? players[0]?.steamId ?? null);

  useEffect(() => {
    setSelected((current) =>
      current !== null && players.some((player) => player.steamId === current)
        ? current
        : (recommended?.steamId ?? players[0]?.steamId ?? null),
    );
  }, [players, recommended?.steamId]);
  const isSeries = (seriesMapCount ?? 0) >= 2;
  const selectedPlayer = players.find((p) => p.steamId === selected);

  const showMvp = players.some((p) => p.mvps > 0);
  const columns: Column[] = [
    { key: 'rating', label: 'RAT', value: (p) => p.rating.toFixed(2), render: (p) => <RatingCell rating={p.rating} /> },
    { key: 'k', label: 'K', value: (p) => `${p.kills}` },
    { key: 'd', label: 'D', value: (p) => `${p.deaths}` },
    { key: 'a', label: 'A', value: (p) => `${p.assists}` },
    { key: 'pm', label: '+/-', secondary: true, value: (p) => signed(p.kills - p.deaths), tone: (p) => (p.kills - p.deaths >= 0 ? 'text-fg-1' : 'text-fg-3') },
    { key: 'adr', label: 'ADR', secondary: true, value: (p) => `${Math.round(p.adr)}` },
    { key: 'kast', label: 'KAST', secondary: true, value: (p) => `${Math.round(p.kast)}%` },
    { key: 'hs', label: 'HS', secondary: true, value: (p) => `${Math.round(p.hsPct)}%` },
    ...(showMvp ? [{ key: 'mvp', label: 'MVP', secondary: true, value: (p: DemoPlayer) => `${p.mvps}` }] : []),
  ];

  // A fixed flexible player column plus one compact, tabular column per stat.
  // A narrow content column (a snapped window, or the picker at mobile width)
  // drops the secondary columns instead of letting the grid crush the player
  // name to zero width, so the template switches on the same container step
  // that hides the cells. Typed rather than cast: CSSProperties has no index
  // signature for custom properties.
  const coreCount = columns.filter((c) => !c.secondary).length;
  const gridStyle: CSSProperties & { '--pp-cols': string; '--pp-cols-wide': string } = {
    '--pp-cols': `minmax(0,1fr) repeat(${coreCount}, minmax(2.5rem,2.75rem))`,
    '--pp-cols-wide': `minmax(0,1fr) repeat(${columns.length}, minmax(2.5rem,2.75rem))`,
  };
  const gridClass = '[grid-template-columns:var(--pp-cols)] @[44rem]/upload:[grid-template-columns:var(--pp-cols-wide)]';
  const cellClass = (c: Column) => (c.secondary ? 'hidden @[44rem]/upload:block' : undefined);

  const sides: Array<DemoPlayer['team']> = ['T', 'CT', ''];
  const groups = sides
    .map((side) => players.filter((p) => p.team === side))
    .map((roster, i) => ({ side: sides[i], roster }))
    .filter((g) => g.roster.length > 0);

  // Series mode shows the combined-stats strip; a single match shows its
  // score header when the scan reported one; otherwise neither.
  let header: ReactNode = null;
  if (isSeries) header = <SeriesSummary mapCount={seriesMapCount ?? 0} playerCount={players.length} />;
  else if (match) header = <MatchHeader match={match} />;

  return (
    <div className="flex flex-col gap-5">
      {header}
      {groups.map(({ side, roster }) => {
        const meta = TEAM_META[side];
        const avg = roster.reduce((s, p) => s + p.rating, 0) / roster.length;
        return (
          <section key={side || 'other'}>
            <div className="mb-2 flex items-center justify-between gap-3 px-1">
              <span className={cn('font-display text-label font-bold uppercase tracking-widest', meta.text)}>
                {meta.label}
              </span>
              <span className="font-mono text-meta uppercase tracking-wider tabular-nums text-fg-3">
                media {avg.toFixed(2)}
              </span>
            </div>

            <div className="overflow-hidden border border-border">
              <div
                className={cn(
                  'grid items-center gap-x-1 border-b border-border bg-surface-3 px-3 py-2 font-mono text-meta uppercase tracking-wider text-fg-3',
                  gridClass,
                )}
                style={gridStyle}
              >
                <span>Jugador</span>
                {columns.map((c) => (
                  <span key={c.key} className={cn('text-right', cellClass(c))} title={STAT_TOOLTIPS[c.key]}>
                    {c.label}
                  </span>
                ))}
              </div>

              {roster.map((p) => {
                const active = p.steamId === selected;
                const isRecommended = p.steamId === recommended?.steamId;
                const chips = highlightChips(p);
                // Series mode: flag players who did not appear in every map. A
                // player present everywhere (the common case) stays visually
                // quiet — no chip at all.
                const showMapsChip =
                  isSeries && typeof p.mapsPresent === 'number' && p.mapsPresent < (seriesMapCount ?? 0);
                const mapsChip = showMapsChip ? (
                  <StatusTag className="tabular-nums">
                    {p.mapsPresent}/{seriesMapCount} mapas
                  </StatusTag>
                ) : null;
                let chipRow: ReactNode;
                if (chips.length > 0) {
                  chipRow = chips.map((c) => (
                    <StatusTag key={c.key} tone={c.tone} className="tabular-nums">
                      {c.label}
                    </StatusTag>
                  ));
                } else if (!isRecommended && !showMapsChip) {
                  chipRow = <span className="font-mono text-meta text-fg-3">—</span>;
                } else {
                  chipRow = null;
                }
                return (
                  <button
                    key={p.steamId}
                    type="button"
                    aria-pressed={active}
                    onClick={() => setSelected(p.steamId)}
                    style={gridStyle}
                    className={cn(
                      'grid w-full cursor-pointer items-center gap-x-1 border-b border-border-subtle px-3 py-2.5 text-left last:border-b-0',
                      'transition-[background-color,box-shadow] duration-(--dur-fast) ease-standard',
                      // The row lives inside an `overflow-hidden` table, so the
                      // ring is drawn INSIDE the row's own box: a positive
                      // outline offset would be clipped away on the first and
                      // last rows. v3 shipped `focus-visible:bg-primary/10`,
                      // which measured ~1.3:1 against the row — no indicator at
                      // all on the most keyboard-driven surface in the product.
                      FOCUS_RING,
                      'focus-visible:-outline-offset-2',
                      gridClass,
                      // The recommended row keeps a permanent left accent + tint so it reads at a
                      // glance even when the user's mouse is elsewhere; the ring below layers on
                      // top for whichever row is the current pick target (hover/keyboard focus).
                      isRecommended && 'bg-primary/10 shadow-[inset_3px_0_0_0_var(--primary)]',
                      active
                        ? 'bg-primary/10 ring-1 ring-inset ring-primary/60'
                        : !isRecommended && 'hover:bg-surface-3',
                    )}
                  >
                    <span className="flex min-w-0 flex-col gap-1">
                      <span className="flex min-w-0 items-center gap-2.5">
                        <span
                          data-testid="player-avatar"
                          className={cn(
                            'inline-flex size-7 shrink-0 items-center justify-center border font-display text-meta font-bold leading-none tracking-normal',
                            active ? 'border-primary/55 bg-primary/15 text-primary' : meta.chip,
                          )}
                        >
                          {initials(p.name)}
                        </span>
                        {/* min-w-0 lets this shrink inside the flex row without evicting the name;
                            the name never has to share a row with the Recommended badge, which
                            keeps this row narrow-window-safe like the rest of the scoreboard. */}
                        <span className="min-w-0 flex-1 truncate text-body-sm font-medium text-fg-1">{p.name}</span>
                      </span>
                      {/* Second line, indented under the name: the Recommended tag and the
                          Highlights chips. Always rendered (never just for narrow windows) so
                          it never competes with the player name for horizontal space. */}
                      <span className="flex flex-wrap items-center gap-1.5 pl-[2.375rem]">
                        {isRecommended ? (
                          <Badge shape="square" className="min-h-7 px-2">
                            Recomendado
                          </Badge>
                        ) : null}
                        {mapsChip}
                        {chipRow}
                      </span>
                    </span>
                    {columns.map((c) => (
                      <span
                        key={c.key}
                        className={cn(
                          'text-right font-mono text-body-sm tabular-nums',
                          cellClass(c),
                          c.tone?.(p) ?? 'text-fg-1',
                        )}
                      >
                        {c.render ? c.render(p) : c.value(p)}
                      </span>
                    ))}
                  </button>
                );
              })}
            </div>
          </section>
        );
      })}

      {/*
        The confirm bar. It bleeds to the card's inner edge (the card pads
        `p-4` / `p-6`), so nothing scrolls past it through an unbled gutter, and
        it is opaque rather than `backdrop-blur`: a blurred sticky band re-reads
        and two-pass blurs its whole strip on every scroll frame, and on an
        opaque navy card the two are indistinguishable.
      */}
      <div className="sticky bottom-0 z-20 -mx-4 -mb-4 flex flex-wrap items-center justify-between gap-4 rounded-b-[calc(var(--radius)-1px)] border-t border-border-accent bg-surface-0 px-4 py-3 shadow-[0_-12px_28px_-18px_oklch(0.02_0.02_264/0.9)] @[40rem]/upload:-mx-6 @[40rem]/upload:-mb-6 @[40rem]/upload:px-6">
        <p className="min-w-0 text-body-sm text-fg-2">
          {selectedPlayer ? (
            <>
              Vas a clipear a <strong className="font-semibold text-fg-1">{selectedPlayer.name}</strong>.
            </>
          ) : (
            'Selecciona un jugador y continúa cuando estés listo.'
          )}
        </p>
        <Button
          type="button"
          variant="hero"
          disabled={selected === null}
          onClick={() => selected && onPick(selected)}
        >
          CONTINUAR
        </Button>
      </div>
    </div>
  );
}
