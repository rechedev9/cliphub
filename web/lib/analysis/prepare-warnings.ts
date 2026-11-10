import type { PrepWarning } from '../api/first-death.ts';

/** A player of the team's roster, as far as the warnings care. */
export type RosterSpan = { nick: string; first_played_at: string; last_played_at: string; matches: number };

export type PrepareWarningInput = {
  roster: readonly RosterSpan[];
  /** When each of the team's matches in the period was played, whatever the lineup keeps. */
  periodPlayedAt: readonly string[];
  /** The server's warnings over the kept sample (`tacticalplan.PrepWarning`); null while it is read. */
  sample: readonly PrepWarning[] | null;
  team: string;
};

/** A player with fewer matches than this is a stand-in, not a lineup change. */
const MIN_ROSTER_MATCHES = 3;
const DAY_MS = 86_400_000;

function day(value: string): number | null {
  const time = Date.parse(value);
  return Number.isNaN(time) ? null : Math.floor(time / DAY_MS);
}

function days(playedAt: readonly string[]): number[] {
  return playedAt.map(day).filter((value) => value !== null);
}

/** Who joined after the period's first match and who stopped before its last one; null when nobody did. */
function lineupChange(roster: readonly RosterSpan[], playedAt: readonly string[]): string | null {
  const period = days(playedAt);
  if (period.length === 0) return null;
  const oldest = Math.min(...period);
  const newest = Math.max(...period);
  const regulars = roster.filter((player) => player.matches >= MIN_ROSTER_MATCHES);
  // Only who played inside the period counts: someone gone before it began changed nothing in it.
  const inside = (value: number | null): value is number => value !== null && value >= oldest && value <= newest;
  const joined = regulars.filter((player) => {
    const first = day(player.first_played_at);
    return inside(first) && first > oldest;
  });
  const left = regulars.filter((player) => {
    const last = day(player.last_played_at);
    return inside(last) && last < newest;
  });
  if (joined.length === 0 && left.length === 0) return null;
  const nicks = (players: readonly RosterSpan[]): string => players.map((player) => player.nick).join(', ');
  const parts = [
    joined.length > 0 ? `${nicks(joined)} ${joined.length === 1 ? 'entró' : 'entraron'} después de la primera partida` : '',
    left.length > 0 ? `${nicks(left)} no ${left.length === 1 ? 'juega' : 'juegan'} las últimas` : '',
  ].filter((part) => part !== '');
  return `La alineación cambió en este periodo: ${parts.join('; ')}.`;
}

/** One server warning as the line on top of Preparar says it. */
export function sampleWarningText(warning: PrepWarning, team: string): string {
  switch (warning.kind) {
    case 'stale':
      return `La última partida de ${team} fue hace ${warning.days} días: puede que hayan cambiado de playbook`;
    case 'lineup':
      return `Alineación distinta en ${warning.changed} de ${warning.matches} partidas`;
    case 'small_sample':
      return `Muestra pequeña: ${warning.rounds} rondas`;
  }
}

/**
 * What a coach must know before trusting a preparation: the server's warnings
 * over the kept sample, then who joined or left inside the period, whichever lineup it counts.
 */
export function prepareWarnings({ roster, periodPlayedAt, sample, team }: PrepareWarningInput): string[] {
  const out = (sample ?? []).map((warning) => sampleWarningText(warning, team));
  const change = lineupChange(roster, periodPlayedAt);
  if (change !== null) out.push(change);
  return out;
}
