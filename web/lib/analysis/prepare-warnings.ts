/** A player of the team's roster, as far as the warnings care. */
export type RosterSpan = { nick: string; first_played_at: string; last_played_at: string; matches: number };

export type PrepareWarningInput = {
  roster: readonly RosterSpan[];
  /** When each match of the sample was played. */
  playedAt: readonly string[];
  now: number;
};

/** Matches older than this no longer describe how the team plays today. */
export const STALE_DAYS = 60;
/** A player with fewer matches than this is a stand-in, not a lineup change. */
const MIN_ROSTER_MATCHES = 3;
const DAY_MS = 86_400_000;

function day(value: string): number | null {
  const time = Date.parse(value);
  return Number.isNaN(time) ? null : Math.floor(time / DAY_MS);
}

/**
 * What a coach must know before trusting a preparation: the sample is old, or
 * the lineup changed inside it (someone joined after it starts or left before it ends).
 */
export function prepareWarnings({ roster, playedAt, now }: PrepareWarningInput): string[] {
  const days = playedAt.map(day).filter((value) => value !== null);
  if (days.length === 0) return [];
  const oldest = Math.min(...days);
  const newest = Math.max(...days);
  const out: string[] = [];
  const age = Math.floor(now / DAY_MS) - newest;
  if (age > STALE_DAYS) out.push(`La partida más reciente de esta muestra es de hace ${age} días: puede que ya no jueguen así.`);
  const regulars = roster.filter((player) => player.matches >= MIN_ROSTER_MATCHES);
  const joined = regulars.filter((player) => (day(player.first_played_at) ?? oldest) > oldest).map((player) => player.nick);
  const left = regulars.filter((player) => (day(player.last_played_at) ?? newest) < newest).map((player) => player.nick);
  if (joined.length > 0 || left.length > 0) {
    const parts = [joined.length > 0 ? `${joined.join(', ')} ${joined.length === 1 ? 'entró' : 'entraron'} después de la primera partida` : '', left.length > 0 ? `${left.join(', ')} no ${left.length === 1 ? 'juega' : 'juegan'} las últimas` : ''].filter((part) => part !== '');
    out.push(`La alineación cambió dentro de la muestra: ${parts.join('; ')}.`);
  }
  return out;
}
