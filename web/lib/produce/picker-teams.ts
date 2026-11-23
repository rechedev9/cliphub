import type { LibraryMatch } from '../api/library.ts';
import type { DemoPlayer, RosterMatch } from '../api/types.ts';
import { teamByKey, teamLabel } from '../library/partidas.ts';

type Side = DemoPlayer['team'];

export type PickerTeam<P> = {
  readonly key: string;
  readonly side: Side;
  readonly name: string | null;
  readonly startedAs: string | null;
  readonly score: number | null;
  readonly roster: P[];
};

export type PickerTeamsInput<P> = {
  players: readonly P[];
  match: RosterMatch | null;
  library: LibraryMatch | null;
};

const SIDES: readonly Side[] = ['T', 'CT', ''];

function bySide<P extends { readonly team: Side }>({ players, match }: PickerTeamsInput<P>): PickerTeam<P>[] {
  return SIDES.map((side) => {
    let score: number | null = null;
    if (match !== null && side === 'T') score = match.scoreT;
    if (match !== null && side === 'CT') score = match.scoreCt;
    return { key: side || 'other', side, name: null, startedAs: null, score, roster: players.filter((p) => p.team === side) };
  }).filter((team) => team.roster.length > 0);
}

export function pickerTeams<P extends { readonly steamId: string; readonly team: Side }>(input: PickerTeamsInput<P>): PickerTeam<P>[] {
  const { players, library } = input;
  const keyOf = new Map((library?.players ?? []).map((player) => [player.steamid64, player.team_key]));
  if (library === null || players.length === 0 || players.some((player) => !keyOf.has(player.steamId))) return bySide(input);
  const teams: PickerTeam<P>[] = [];
  for (const key of ['ct-start', 't-start'] as const) {
    const team = teamByKey(library, key);
    const roster = players.filter((player) => keyOf.get(player.steamId) === key);
    if (team === undefined || roster.length === 0) continue;
    teams.push({ key, side: team.start_side, name: teamLabel(team), startedAs: `Empezó de ${team.start_side}`, score: team.score, roster });
  }
  return teams;
}

export type PickerScoreSide = { readonly score: number; readonly side: Side };

export function pickerScore({ match, library }: { match: RosterMatch | null; library: LibraryMatch | null }): readonly [PickerScoreSide, PickerScoreSide] | null {
  const ct = library === null ? undefined : teamByKey(library, 'ct-start');
  const t = library === null ? undefined : teamByKey(library, 't-start');
  if (ct !== undefined && t !== undefined) return [{ score: ct.score, side: ct.start_side }, { score: t.score, side: t.start_side }];
  return match === null ? null : [{ score: match.scoreT, side: 'T' }, { score: match.scoreCt, side: 'CT' }];
}
