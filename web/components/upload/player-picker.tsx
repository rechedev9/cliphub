'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { ArrowRight, Users } from 'lucide-react';
import type { DemoPlayer, RosterMatch } from '@/lib/api/types';
import type { FaceitScoreboard } from '@/lib/api/faceit-scoreboard';
import { prettyMapName } from '@/lib/format';
import { MAX_POV_PLAYERS, MIN_POV_PLAYERS, togglePovPick } from '@/lib/produce/multi-pov';
import { cn } from '@/lib/utils';
import { FaceitRoster } from '@/components/upload/faceit-roster';
import { RosterTeam, type PickerPlayer } from '@/components/upload/roster-team';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { StatusTag } from '@/components/studio/status-tag';
import { StudioDataRow } from '@/components/studio/data-row';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';

export type PlayerPickerProps = {
  /** Roster from the scan (single match) or aggregated across a series, sorted by kills desc. */
  players: PickerPlayer[];
  /** Fires when the user confirms the currently selected target. */
  onPick: (steamId: string, destination?: 'highlights' | 'full-demo') => void;
  /** Match-level context (map, score, rounds) shown above the tables, when the scan has it. */
  match?: RosterMatch;
  /** Two or more maps switch the picker to its aggregated-series presentation. */
  seriesMapCount?: number;
  /** Full-demo selects a continuous POV, while highlights optimizes for clip-worthy rounds. */
  purpose?: 'highlights' | 'full-demo';
  /** "Cancelar" hands the way back to the host; omitted when there is none. */
  onCancel?: () => void;
  allowDestinationSwitch?: boolean;
  /** FACEIT room scoreboard of this demo; replaces the demo's own tables so the numbers match the room. */
  faceit?: FaceitScoreboard | null;
  /** Offers "Varios del equipo": 2 to 5 teammates for one video, the first pick leading. */
  onPickMany?: (steamIds: string[]) => void;
  /** The flow colour of the selection and the CTA; defaults from `purpose`. */
  tone?: FormatTone;
};

type PickMode = 'one' | 'team';
const PICK_MODES = [{ value: 'one', label: 'Un jugador' }, { value: 'team', label: 'Varios del equipo' }] as const;

const MODE_HINT = {
  one: 'El vídeo sigue sus jugadas desde su vista.',
  team: 'De 2 a 5 del mismo equipo: sus mejores jugadas, cada una desde su vista.',
} as const satisfies Record<PickMode, string>;

/** Clip-worthiness weights higher multi-kill rounds most strongly. */
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

/** Highest match rating for a continuous full-demo POV, tiebroken by kills. */
function pickPovRecommended(players: DemoPlayer[]): DemoPlayer | undefined {
  return players.reduce<DemoPlayer | undefined>((best, player) => {
    if (!best) return player;
    return player.rating > best.rating || (player.rating === best.rating && player.kills > best.kills)
      ? player
      : best;
  }, undefined);
}

// Series has no single score, so its header shows map and roster counts instead.
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

/** The match above the two sides: map, final score in the winner's colour, rounds played. */
function MatchHeader({ match }: { match: RosterMatch }): ReactNode {
  const tWon = match.scoreT > match.scoreCt;
  const ctWon = match.scoreCt > match.scoreT;
  return (
    <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1 rounded-[14px] bg-surface-2 px-4 py-3">
      <span className="min-w-0 truncate font-display text-title font-extrabold text-fg-1">{prettyMapName(match.map)}</span>
      <span role="img" aria-label={`Marcador ${match.scoreT} a ${match.scoreCt}`} className="text-title font-extrabold tabular-nums">
        <span className={tWon ? 'text-side-t' : 'text-fg-3'}>{match.scoreT}</span>
        <span className="text-fg-3"> : </span>
        <span className={ctWon ? 'text-side-ct' : 'text-fg-3'}>{match.scoreCt}</span>
      </span>
      <span className="text-meta text-fg-3">{match.rounds} rondas</span>
    </div>
  );
}

// Highlights recommends multi-kill potential; Full Demo recommends the strongest match rating for a continuous POV.
export function PlayerPicker({
  players,
  onPick,
  match,
  seriesMapCount,
  purpose = 'highlights',
  allowDestinationSwitch = true,
  onCancel,
  faceit,
  onPickMany,
  tone = purpose === 'full-demo' ? 'full' : 'short',
}: PlayerPickerProps): ReactNode {
  const recommended = purpose === 'full-demo' ? pickPovRecommended(players) : pickRecommended(players);
  const [selected, setSelected] = useState<string | null>(recommended?.steamId ?? players[0]?.steamId ?? null);
  const [mode, setMode] = useState<PickMode>('one');
  const [team, setTeam] = useState<PickerPlayer[]>([]);

  useEffect(() => {
    setSelected((current) =>
      current !== null && players.some((player) => player.steamId === current)
        ? current
        : (recommended?.steamId ?? players[0]?.steamId ?? null),
    );
    setTeam((current) => current.filter((p) => players.some((player) => player.steamId === p.steamId)));
  }, [players, recommended?.steamId]);
  const isSeries = (seriesMapCount ?? 0) >= 2;
  const canPickMany = onPickMany !== undefined && !isSeries;
  const multi = canPickMany && mode === 'team';
  const byId = new Map(players.map((p) => [p.steamId, p]));
  const selectedPlayer = players.find((p) => p.steamId === selected);
  let chosenIds: string[] = [];
  if (multi) chosenIds = team.map((p) => p.steamId);
  else if (selected !== null) chosenIds = [selected];
  const chosen = new Set(chosenIds);
  const choose = (steamId: string) => {
    const player = byId.get(steamId);
    if (!multi) setSelected(steamId);
    else if (player) setTeam((current) => togglePovPick(current, player));
  };
  const pickTeam = (steamIds: readonly string[]) =>
    setTeam(steamIds.flatMap((id) => byId.get(id) ?? []).slice(0, MAX_POV_PLAYERS));
  const switchMode = (next: PickMode) => {
    if (next === 'team' && team.length === 0 && selectedPlayer) setTeam([selectedPlayer]);
    setMode(next);
  };
  // Series stats are aggregated from the demos, so FACEIT only replaces a single map.
  const faceitBoard = !isSeries && faceit ? faceit : null;
  const faceitPlayers = faceitBoard?.teams.flatMap((t) => t.players) ?? [];
  const faceitSelected = faceitPlayers.find((p) => p.steamId === selected);
  const selectable = new Set(players.map((p) => p.steamId));
  const recommendedLabel = purpose === 'full-demo' ? 'Mejor rendimiento' : 'Recomendado';
  let contextLabel = 'Demo';
  if (isSeries) contextLabel = `Serie · ${seriesMapCount} mapas`;
  else if (match) contextLabel = prettyMapName(match.map);
  let ctaLabel = purpose === 'full-demo' ? 'Continuar a la demo completa' : 'Continuar al Short';
  if (isSeries) ctaLabel = 'Continuar con la serie';
  if (multi) ctaLabel = team.length >= MIN_POV_PLAYERS ? `Analizar ${team.length} jugadores` : 'Analizar jugadores';
  const nameOf = (p: PickerPlayer) => faceitPlayers.find((f) => f.steamId === p.steamId)?.nickname ?? p.name;

  let footLabel = 'Jugador elegido';
  let footName = 'Elige un jugador para continuar';
  let footHint: string | null = null;
  if (multi) {
    footLabel = team.length > 0 ? `${team.length} ${team.length === 1 ? 'jugador' : 'jugadores'}` : 'Jugadores elegidos';
    footName = team.length > 0 ? team.map(nameOf).join(', ') : 'Elige de 2 a 5 jugadores del mismo equipo';
    if (team.length === 1) footHint = 'Añade otro jugador de su equipo';
  } else if (selectedPlayer) {
    footName = faceitSelected?.nickname ?? selectedPlayer.name;
  }
  const footStats = !multi && selectedPlayer ? (
    <div className="flex gap-7">
      <FootStat value={faceitSelected?.kills ?? selectedPlayer.kills} label="kills" className={FORMAT_TONE[tone].text} />
      <FootStat value={faceitSelected?.deaths ?? selectedPlayer.deaths} label="muertes" />
      {selectedPlayer.rounds5k ? <FootStat value={selectedPlayer.rounds5k} label="ACE" /> : null}
    </div>
  ) : null;

  const showMvp = players.some((p) => p.mvps > 0);
  const sides: Array<DemoPlayer['team']> = ['T', 'CT', ''];
  const groups = sides
    .map((side) => ({ side, roster: players.filter((p) => p.team === side) }))
    .filter((g) => g.roster.length > 0);
  const sideScore = (side: DemoPlayer['team']): number | null => {
    if (isSeries || !match || side === '') return null;
    return side === 'T' ? match.scoreT : match.scoreCt;
  };

  return (
    <div className="flex flex-col gap-5">
      {canPickMany ? (
        <div className="flex flex-wrap items-center gap-x-3.5 gap-y-2">
          <Segmented label="Jugadores del vídeo" value={mode} options={PICK_MODES} onChange={switchMode} />
          <span className="text-body-sm text-fg-2">{MODE_HINT[mode]}</span>
        </div>
      ) : null}
      {faceitBoard ? (
        <FaceitRoster
          board={faceitBoard}
          selected={chosen}
          onSelect={choose}
          onSelectTeam={multi ? pickTeam : undefined}
          selectable={selectable}
          recommended={recommended?.steamId}
          recommendedLabel={recommendedLabel}
        />
      ) : null}
      {!faceitBoard && isSeries ? <SeriesSummary mapCount={seriesMapCount ?? 0} playerCount={players.length} /> : null}
      {!faceitBoard && !isSeries && match ? <MatchHeader match={match} /> : null}
      {faceitBoard ? null : (
        <div className="grid items-start gap-5 @[60rem]/content:grid-cols-2">
          {groups.map(({ side, roster }) => (
            <RosterTeam
              key={side || 'other'}
              side={side}
              roster={roster}
              score={sideScore(side)}
              tone={tone}
              chosen={chosen}
              recommendedId={recommended?.steamId}
              recommendedLabel={recommendedLabel}
              seriesMapCount={isSeries ? (seriesMapCount ?? 0) : null}
              multi={multi}
              showMvp={showMvp}
              onChoose={choose}
              onPickTeam={multi && side !== '' ? () => pickTeam(roster.map((p) => p.steamId)) : undefined}
            />
          ))}
        </div>
      )}

      <div className="sticky bottom-0 z-20 flex flex-wrap items-center gap-x-6 gap-y-3 rounded-2xl border border-border bg-surface-1 px-5 py-4 shadow-[var(--elev-4)]">
        <div className="flex min-w-44 flex-1 flex-col gap-0.5 @[48rem]/content:flex-none">
          <span className="text-meta text-fg-3">{contextLabel} · {footLabel}</span>
          <p className={cn('truncate font-display text-title font-extrabold', multi && team.length < MIN_POV_PLAYERS ? 'text-fg-2' : 'text-fg-1')}>
            {footName}
          </p>
          {footHint ? <span className="text-meta text-fg-3">{footHint}</span> : null}
        </div>
        {footStats}
        <span aria-hidden className="hidden flex-1 @[48rem]/content:block" />
        <div className="flex flex-wrap items-center gap-2">
          {onCancel !== undefined ? (
            <Button type="button" variant="ghost" onClick={onCancel}>
              Cancelar
            </Button>
          ) : null}
          {allowDestinationSwitch && purpose === 'highlights' && !isSeries && !multi ? (
            <Button
              type="button"
              variant="outline"
              disabled={selected === null}
              onClick={() => selected && onPick(selected, 'full-demo')}
            >
              Preparar demo completa
            </Button>
          ) : null}
          <Button
            type="button"
            size="lg"
            className={cn('rounded-xl font-bold', FORMAT_TONE[tone].fill)}
            disabled={multi ? team.length < MIN_POV_PLAYERS : selected === null}
            onClick={() => {
              if (multi) onPickMany?.(team.map((p) => p.steamId));
              else if (selected) onPick(selected, purpose);
            }}
          >
            {ctaLabel}
            <ArrowRight aria-hidden />
          </Button>
        </div>
      </div>
    </div>
  );
}

function FootStat({ value, label, className }: { value: number; label: string; className?: string }): ReactNode {
  return (
    <span className="flex flex-col">
      <span className={cn('font-display text-[1.875rem] font-extrabold leading-none tracking-[-0.02em] tabular-nums text-fg-1', className)}>{value}</span>
      <span className="text-meta text-fg-3">{label}</span>
    </span>
  );
}
