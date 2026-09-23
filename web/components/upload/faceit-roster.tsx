'use client';

import type { CSSProperties, ReactNode } from 'react';
import { ExternalLink } from 'lucide-react';
import type { FaceitScoreboard, FaceitScoreboardPlayer, FaceitScoreboardTeam } from '@/lib/api/faceit-scoreboard';
import { prettyMapName } from '@/lib/format';
import { cn } from '@/lib/utils';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { StatusTag } from '@/components/studio/status-tag';
import { FOCUS_RING } from '@/components/ui/button';

// FACEIT CS2 level floors: level n starts at LEVEL_FLOORS[n - 1] ELO.
const LEVEL_FLOORS = [100, 501, 751, 901, 1051, 1201, 1351, 1531, 1751, 2001] as const;

export function levelFromElo(elo: number): number {
  let level = 1;
  LEVEL_FLOORS.forEach((floor, i) => {
    if (elo >= floor) level = i + 1;
  });
  return level;
}

function levelTone(level: number): string {
  if (level >= 8) return 'text-destructive';
  if (level >= 4) return 'text-warning';
  if (level >= 2) return 'text-success';
  return 'text-fg-2';
}

/** FACEIT's level gauge: a ring filled to level/10 around the level number. */
export function FaceitLevel({ level, size = 26 }: { level?: number; size?: number }): ReactNode {
  const r = 10;
  const circumference = 2 * Math.PI * r;
  const filled = level === undefined ? 0 : Math.min(level, 10) / 10;
  return (
    <span
      role="img"
      aria-label={`Nivel FACEIT ${level ?? 'desconocido'}`}
      title={`Nivel FACEIT ${level ?? 'desconocido'}`}
      className={cn('relative inline-grid shrink-0 place-items-center', level === undefined ? 'text-fg-3' : levelTone(level))}
      style={{ width: size, height: size }}
    >
      <svg viewBox="0 0 24 24" className="absolute inset-0 size-full -rotate-90" aria-hidden>
        <circle cx="12" cy="12" r={r} fill="var(--surface-0)" stroke="var(--border-subtle)" strokeWidth="2.5" />
        <circle
          cx="12"
          cy="12"
          r={r}
          fill="none"
          stroke="currentColor"
          strokeWidth="2.5"
          strokeDasharray={`${circumference * filled} ${circumference}`}
        />
      </svg>
      <span className="relative text-[0.6875rem] font-bold leading-none tabular-nums">{level ?? '–'}</span>
    </span>
  );
}

type Column = {
  key: string;
  label: string;
  title?: string;
  /** 0 always shows; 1 and 2 appear as the content column widens. */
  tier: 0 | 1 | 2;
  width: string;
  value: (p: FaceitScoreboardPlayer) => string;
  tone?: (p: FaceitScoreboardPlayer) => string | undefined;
};

const dimZero = (n: number) => (n === 0 ? 'text-fg-3' : undefined);

// Same columns and order as the FACEIT room scoreboard. FACEIT's own rating
// and Swing are not part of the public Data API, so they are left out rather
// than replaced with a stat the room does not show.
const COLUMNS: Column[] = [
  { key: 'k', label: 'K', tier: 0, width: '2.5rem', value: (p) => `${p.kills}` },
  { key: 'd', label: 'D', tier: 0, width: '2.5rem', value: (p) => `${p.deaths}` },
  { key: 'a', label: 'A', tier: 0, width: '2.5rem', value: (p) => `${p.assists}` },
  { key: 'adr', label: 'ADR', title: 'Daño medio por ronda', tier: 0, width: '3.25rem', value: (p) => p.adr.toFixed(1) },
  { key: 'kd', label: 'K/D', tier: 1, width: '3rem', value: (p) => p.kd.toFixed(2) },
  { key: 'kr', label: 'K/R', title: 'Kills por ronda', tier: 1, width: '3rem', value: (p) => p.kr.toFixed(2) },
  { key: 'hs', label: 'HS', title: 'Kills por headshot', tier: 2, width: '2.5rem', value: (p) => `${p.headshots}` },
  { key: 'hsp', label: 'HS %', tier: 1, width: '3.5rem', value: (p) => `${p.hsPct.toFixed(1)}%` },
  { key: '5k', label: '5K', tier: 2, width: '2.25rem', value: (p) => `${p.rounds5k}`, tone: (p) => dimZero(p.rounds5k) },
  { key: '4k', label: '4K', tier: 2, width: '2.25rem', value: (p) => `${p.rounds4k}`, tone: (p) => dimZero(p.rounds4k) },
  { key: '3k', label: '3K', tier: 2, width: '2.25rem', value: (p) => `${p.rounds3k}`, tone: (p) => dimZero(p.rounds3k) },
  { key: '2k', label: '2K', tier: 2, width: '2.25rem', value: (p) => `${p.rounds2k}`, tone: (p) => dimZero(p.rounds2k) },
  { key: 'mvp', label: 'MVPs', tier: 1, width: '3rem', value: (p) => `${p.mvps}` },
];

function template(maxTier: number): string {
  const stats = COLUMNS.filter((c) => c.tier <= maxTier).map((c) => c.width);
  return ['minmax(9rem,1fr)', '5.75rem', ...stats].join(' ');
}

const GRID_STYLE: CSSProperties & Record<'--fr-0' | '--fr-1' | '--fr-2', string> = {
  '--fr-0': template(0),
  '--fr-1': template(1),
  '--fr-2': template(2),
};
const GRID_CLASS =
  '[grid-template-columns:var(--fr-0)] @[48rem]/content:[grid-template-columns:var(--fr-1)] @[62rem]/content:[grid-template-columns:var(--fr-2)]';
const HEAD_TIER = ['', 'hidden @[48rem]/content:block', 'hidden @[62rem]/content:block'] as const;
const CELL_TIER = ['flex', 'hidden @[48rem]/content:flex', 'hidden @[62rem]/content:flex'] as const;

// FACEIT tells the two factions apart by colour; the theme's cyan and green
// keep that without borrowing FACEIT's palette.
const TEAM_TINTS = [
  { cell: 'bg-primary/12', bar: 'bg-primary' },
  { cell: 'bg-success/10', bar: 'bg-success' },
] as const;

export type FaceitRosterProps = {
  board: FaceitScoreboard;
  selected: string | null;
  onSelect: (steamId: string) => void;
  /** Steam IDs present in the parsed demo; any other row cannot be picked. */
  selectable: ReadonlySet<string>;
  recommended?: string;
  recommendedLabel: string;
};

/** The FACEIT room scoreboard as a POV picker: one table per team, winner first. */
export function FaceitRoster({ board, selected, onSelect, selectable, recommended, recommendedLabel }: FaceitRosterProps): ReactNode {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3 border border-border bg-surface-2 px-3.5 py-2.5">
        <span className="text-body-sm text-fg-2">
          Estadísticas oficiales de la partida en FACEIT · {prettyMapName(board.map)} · {board.rounds} rondas
        </span>
        <a
          href={board.roomUrl}
          target="_blank"
          rel="noreferrer"
          className={cn('inline-flex items-center gap-1.5 text-body-sm font-medium text-primary hover:text-fg-1', FOCUS_RING)}
        >
          Abrir sala <ExternalLink aria-hidden className="size-3.5" />
        </a>
      </div>
      {board.teams.map((team, i) => (
        <TeamTable
          key={team.name || i}
          team={team}
          tint={TEAM_TINTS[i % TEAM_TINTS.length]}
          selected={selected}
          onSelect={onSelect}
          selectable={selectable}
          recommended={recommended}
          recommendedLabel={recommendedLabel}
        />
      ))}
    </div>
  );
}

function TeamTable({
  team,
  tint,
  selected,
  onSelect,
  selectable,
  recommended,
  recommendedLabel,
}: Omit<FaceitRosterProps, 'board'> & { team: FaceitScoreboardTeam; tint: (typeof TEAM_TINTS)[number] }): ReactNode {
  return (
    <section aria-label={team.name}>
      <header className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 px-1">
        <span className="flex min-w-0 items-center gap-3">
          <span className={cn('text-title font-bold tabular-nums', team.won ? 'text-success' : 'text-fg-2')}>{team.score}</span>
          <span aria-hidden className={cn('h-5 w-1', tint.bar)} />
          <span className="truncate font-display text-body font-bold text-fg-1">{team.name}</span>
        </span>
        <span className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1 text-body-sm text-fg-3">
          {team.averageElo !== undefined ? (
            <span className="flex items-center gap-2">
              Promedio del equipo
              <FaceitLevel level={levelFromElo(team.averageElo)} size={22} />
              <span className="font-bold tabular-nums text-fg-1">{team.averageElo}</span>
            </span>
          ) : null}
          <span aria-hidden className="hidden h-4 w-px bg-border @[40rem]/content:block" />
          <span>
            Primera mitad <span className="font-bold tabular-nums text-fg-1">{team.firstHalf}</span>
          </span>
          <span>
            Segunda mitad <span className="font-bold tabular-nums text-fg-1">{team.secondHalf}</span>
          </span>
          {team.overtime > 0 ? (
            <span>
              Prórroga <span className="font-bold tabular-nums text-fg-1">{team.overtime}</span>
            </span>
          ) : null}
        </span>
      </header>

      <div className="overflow-hidden border border-border bg-surface-2">
        <div
          className={cn('grid items-center border-b border-border bg-surface-3 text-meta font-semibold tracking-normal text-fg-2', GRID_CLASS)}
          style={GRID_STYLE}
        >
          <span className="px-3 py-2">Jugador</span>
          <span className="px-2 py-2">Rango</span>
          {COLUMNS.map((c) => (
            <span key={c.key} title={c.title} className={cn('px-2 py-2 text-left', HEAD_TIER[c.tier])}>
              {c.label}
            </span>
          ))}
        </div>

        {team.players.map((p) => {
          const pickable = p.steamId !== undefined && selectable.has(p.steamId);
          const active = pickable && p.steamId === selected;
          const isRecommended = pickable && p.steamId === recommended;
          return (
            <button
              key={p.playerId}
              type="button"
              disabled={!pickable}
              aria-pressed={active}
              title={pickable ? undefined : 'Este jugador no aparece en la demo'}
              onClick={() => p.steamId && onSelect(p.steamId)}
              style={GRID_STYLE}
              className={cn(
                'grid w-full items-stretch border-b border-border-subtle text-left last:border-b-0',
                'transition-[background-color,box-shadow] duration-(--dur-fast) ease-standard',
                FOCUS_RING,
                'focus-visible:-outline-offset-2',
                GRID_CLASS,
                pickable ? 'cursor-pointer hover:bg-surface-3' : 'cursor-not-allowed opacity-50',
                active && 'bg-surface-3 ring-2 ring-inset ring-primary',
              )}
            >
              <span className={cn('flex min-w-0 items-center gap-2.5 px-3 py-2.5', tint.cell)}>
                <PlayerAvatar nickname={p.nickname} playerID={p.playerId} avatar={p.avatar} size={32} />
                <span className="min-w-0 truncate text-body-sm font-bold text-fg-1">{p.nickname}</span>
                {isRecommended ? (
                  <StatusTag tone="primary" className="shrink-0">
                    {recommendedLabel}
                  </StatusTag>
                ) : null}
              </span>
              <span className={cn('flex items-center gap-2 px-2', tint.cell)}>
                <FaceitLevel level={p.skillLevel} />
                <span className="text-body-sm tabular-nums text-fg-1">{p.elo ?? '—'}</span>
              </span>
              {COLUMNS.map((c) => (
                <span
                  key={c.key}
                  className={cn(
                    'items-center px-2 text-body-sm tabular-nums',
                    CELL_TIER[c.tier],
                    c.tone?.(p) ?? 'text-fg-1',
                  )}
                >
                  {c.value(p)}
                </span>
              ))}
            </button>
          );
        })}
      </div>
    </section>
  );
}
