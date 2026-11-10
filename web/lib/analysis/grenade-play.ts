import type { GrenadeRound } from '../api/grenades.ts';
import { PLAY_GRENADES, type PlayGrenade, type PlayInput } from '../api/stratbook.ts';
import { TACTICAL_SITE_ORDER, type TacticalSide, type TacticalSite } from '../api/tactical.ts';
import { MAX_PLAY_EXAMPLES } from '../stratbook/plays.ts';
import { factLabel, roundKey, type FollowUp } from './grenade-follow-up.ts';
import { spotReplayRefs, type GrenadeSpot } from './grenade-patterns.ts';

/** How many of the follow-up facts the play's notes name. */
const NOTE_FACTS = 3;

function median(values: readonly number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  if (sorted.length === 0) return 0;
  return sorted.length % 2 === 1 ? (sorted[middle] ?? 0) : ((sorted[middle - 1] ?? 0) + (sorted[middle] ?? 0)) / 2;
}

function commonSite(rounds: readonly GrenadeRound[]): TacticalSite {
  const counts = new Map<TacticalSite, number>();
  for (const round of rounds) {
    const site = TACTICAL_SITE_ORDER.find((entry) => entry === round.site && entry !== 'none');
    if (site !== undefined) counts.set(site, (counts.get(site) ?? 0) + 1);
  }
  let best: TacticalSite = 'none';
  for (const [site, count] of counts) if (count > (counts.get(best) ?? 0)) best = site;
  return best;
}

function playGrenade(spot: GrenadeSpot): PlayGrenade[] {
  const kind = PLAY_GRENADES.find((entry) => entry === spot.kind);
  const first = spot.grenades[0];
  if (kind === undefined || first === undefined) return [];
  return [
    {
      kind,
      seconds: Math.round(median(spot.grenades.map((grenade) => grenade.seconds)) * 10) / 10,
      pos: [spot.landing[0], spot.landing[1], first.landing[2]],
      ...(first.land_place === '' ? {} : { callout: first.land_place }),
    },
  ];
}

/**
 * A Libreta play from the finder's selection (one grenade or several): each
 * grenade where it lands, in throw order, what follows in the notes and the
 * rounds as examples, from the throw. `followUp` is read over every grenade of the selection.
 */
export function playFromGrenadeSpots(options: {
  teamId: string;
  map: string;
  side: TacticalSide;
  title: string;
  spots: readonly GrenadeSpot[];
  rounds: readonly GrenadeRound[];
  followUp: FollowUp;
  place: (token: string) => string;
}): PlayInput {
  const { spots, followUp, place } = options;
  const all = spots.flatMap((spot) => spot.grenades);
  const kept = new Set(all.map((grenade) => roundKey(grenade.match_id, grenade.round)));
  const rounds = options.rounds.filter((round) => kept.has(roundKey(round.match_id, round.round)));
  const grenades = spots.flatMap(playGrenade).sort((a, b) => a.seconds - b.seconds);
  const pct = followUp.rounds > 0 ? Math.round((followUp.won / followUp.rounds) * 100) : 0;
  const after = followUp.facts
    .slice(0, NOTE_FACTS)
    .map((fact) => `${factLabel(fact, place)} (${fact.rounds} de ${followUp.rounds})`)
    .join('; ');
  const lead = spots.length === 1 ? `Granada repetida en ${followUp.rounds} rondas` : `${spots.length} granadas en ${followUp.rounds} rondas`;
  return {
    title: options.title.trim(),
    team_id: options.teamId,
    map: options.map,
    side: options.side,
    kind: options.side === 'T' ? 'default' : 'hold',
    site: commonSite(rounds),
    origin: 'detected',
    positions_seconds: 0,
    grenades,
    positions: [],
    notes: `${lead}; gana el ${pct} %.${after === '' ? '' : ` Después: ${after}.`}`,
    examples: spotReplayRefs(all)
      .slice(0, MAX_PLAY_EXAMPLES)
      .map((ref) => ({ match_id: ref.matchId, round: ref.round, tick: ref.tick ?? 0 })),
  };
}
