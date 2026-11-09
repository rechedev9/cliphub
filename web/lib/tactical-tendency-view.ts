import { TACTICAL_CT_PATTERN_ORDER, TACTICAL_T_PATTERN_ORDER } from './api/tactical.ts';
import type { TacticalFilter, TacticalRate, TacticalRound, TacticalSide, TacticalTendencies } from './api/tactical.ts';
import { tacticalPerspective } from './tactical-filter.ts';
import type { TeamSides } from './tactical-filter.ts';
import { buyLabel, patternLabel, siteLabel } from './tactical-labels.ts';

// The tendencies panel under the replay: whose rounds it reads, the filter it
// asks the aggregate for, and the card rows it draws from the answer.

const PERCENT = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 1 });

/** A percentage in Spanish notation: `56,5 %`. */
export function percentText(pct: number): string {
  return `${PERCENT.format(pct)} %`;
}

/** How a number reads against the outcome colours. */
export type OutcomeTone = 'win' | 'loss' | 'even';

function winTone(rate: TacticalRate): OutcomeTone {
  return rate.pct >= 50 ? 'win' : 'loss';
}

function roundsLabel(rounds: number): string {
  return rounds === 1 ? '1 ronda' : `${rounds} rondas`;
}

type WinnerRound = Pick<TacticalRound, 'half' | 'winner'>;

/** The team that won more of the document's rounds; undefined on a tie or without teams. */
export function matchWinnerKey(teams: readonly TeamSides[], rounds: readonly WinnerRound[]): string | undefined {
  let best: string | undefined;
  let bestWins = -1;
  let tied = false;
  for (const team of teams) {
    const filter: TacticalFilter = { team_key: team.key };
    const wins = rounds.filter((round) => round.winner !== '' && tacticalPerspective(teams, filter, round) === round.winner).length;
    if (wins > bestWins) {
      best = team.key;
      bestWins = wins;
      tied = false;
    } else if (wins === bestWins) {
      tied = true;
    }
  }
  return tied ? undefined : best;
}

/** The one team the filtered players belong to, so a player filter never opens on an empty panel. */
function slotTeamKey(teams: readonly TeamSides[], slots: readonly number[]): string | undefined {
  if (slots.length === 0) return undefined;
  const owners = teams.filter((team) => (team.slots ?? []).some((slot) => slots.includes(slot)));
  return owners.length === 1 ? owners[0]?.key : undefined;
}

export type PanelTeamInput = {
  teams: readonly TeamSides[];
  rounds: readonly WinnerRound[];
  filter: TacticalFilter;
  /** The viewer's pick in the panel's team toggle. */
  chosen: string | undefined;
};

/**
 * The team the panel reads from: the replay filter's team, else the viewer's
 * pick, else a default. Undefined without two teams, where no toggle can help.
 */
export function panelTeamKey({ teams, rounds, filter, chosen }: PanelTeamInput): string | undefined {
  if (filter.team_key !== undefined) return filter.team_key;
  if (teams.length < 2) return undefined;
  if (chosen !== undefined && teams.some((team) => team.key === chosen)) return chosen;
  return slotTeamKey(teams, filter.slots ?? []) ?? matchWinnerKey(teams, rounds) ?? teams[0]?.key;
}

/** The replay filter with the panel's team on top; the replay keeps its own filter. */
export function panelFilter(filter: TacticalFilter, teamKey: string | undefined): TacticalFilter {
  if (teamKey === undefined || filter.team_key !== undefined) return filter;
  return { ...filter, team_key: teamKey };
}

export type SummaryLineInput = { rounds: number; wins: number; filtered: boolean };

/** `Sus 23 rondas de esta partida, con los filtros del replay. Ganó 13 (56,5 %).` */
export function teamSummaryLine({ rounds, wins, filtered }: SummaryLineInput): string {
  const subject = rounds === 1 ? 'Su única ronda' : `Sus ${rounds} rondas`;
  const scope = filtered ? ', con los filtros del replay' : '';
  const pct = rounds > 0 ? (wins / rounds) * 100 : 0;
  return `${subject} de esta partida${scope}. Ganó ${wins} (${percentText(pct)}).`;
}

export type TendencyRow = {
  key: string;
  label: string;
  rounds: number;
  /** What `rounds` is out of, for the hover text. */
  shareText: string;
  win: TacticalRate;
  winText: string;
  tone: OutcomeTone;
  /** Fewer rounds than MIN_RELIABLE_SAMPLE: a fact, not a tendency yet. */
  small: boolean;
  /** What narrows the panel's rounds to this row's, for the link to the Replay. */
  patch: TacticalFilter;
};

export type TendencyCard = { key: string; title: string; hint: string; column: string; rows: TendencyRow[] };

type WinBucket = { rounds: number; share: TacticalRate; win_rate: TacticalRate };

function row({ key, label, bucket, patch }: { key: string; label: string; bucket: WinBucket; patch: TacticalFilter }): TendencyRow {
  return {
    patch,
    key,
    label,
    rounds: bucket.rounds,
    shareText: `${percentText(bucket.share.pct)} de ${roundsLabel(bucket.share.total)}`,
    win: bucket.win_rate,
    winText: percentText(bucket.win_rate.pct),
    tone: winTone(bucket.win_rate),
    small: !bucket.win_rate.reliable,
  };
}

/** The filter of one round shape: the side it is played on and the shape itself. */
export function shapePatch(side: TacticalSide, pattern: string): TacticalFilter {
  if (side === 'T') return { side, t_patterns: TACTICAL_T_PATTERN_ORDER.filter((entry) => entry === pattern) };
  return { side, ct_patterns: TACTICAL_CT_PATTERN_ORDER.filter((entry) => entry === pattern) };
}

function sideRounds(buckets: readonly WinBucket[]): number {
  return buckets.reduce((sum, bucket) => sum + bucket.rounds, 0);
}

/** Economy, site and both sides' round shapes, each row with its rounds and win rate; empty cards are dropped. */
export function tendencyCards(tendencies: TacticalTendencies): TendencyCard[] {
  const cards: TendencyCard[] = [
    {
      key: 'buys',
      title: 'Economía',
      hint: 'Cuánto gana con cada compra',
      column: 'Compra',
      rows: tendencies.buys.filter((bucket) => bucket.rounds > 0).map((bucket) => row({ key: bucket.buy, label: buyLabel(bucket.buy), bucket, patch: { buys: [bucket.buy] } })),
    },
    {
      key: 'sites',
      title: 'Sitio de la ronda',
      hint: 'Dónde se jugó la ronda',
      column: 'Sitio',
      rows: tendencies.sites.filter((bucket) => bucket.rounds > 0).map((bucket) => row({ key: bucket.site, label: siteLabel(bucket.site), bucket, patch: { sites: [bucket.site] } })),
    },
    {
      key: 't',
      title: `Como T · ${roundsLabel(sideRounds(tendencies.t_patterns))}`,
      hint: 'Forma de sus ataques',
      column: 'Forma',
      rows: tendencies.t_patterns.filter((bucket) => bucket.rounds > 0).map((bucket) => row({ key: bucket.pattern, label: patternLabel(bucket.pattern), bucket, patch: shapePatch('T', bucket.pattern) })),
    },
    {
      key: 'ct',
      title: `Como CT · ${roundsLabel(sideRounds(tendencies.ct_patterns))}`,
      hint: 'Forma de sus defensas',
      column: 'Forma',
      rows: tendencies.ct_patterns.filter((bucket) => bucket.rounds > 0).map((bucket) => row({ key: bucket.pattern, label: patternLabel(bucket.pattern), bucket, patch: shapePatch('CT', bucket.pattern) })),
    },
  ];
  return cards.filter((card) => card.rows.length > 0);
}

export type OpeningLine = { key: string; name: string; text: string; tone: OutcomeTone };

/** Each player's opening duels won and lost, most won first. */
export function openingLines(players: TacticalTendencies['players']): OpeningLine[] {
  return [...players]
    .sort((a, b) => b.opening_kills - a.opening_kills || a.opening_deaths - b.opening_deaths || a.name.localeCompare(b.name))
    .map((player) => {
      let tone: OutcomeTone = 'even';
      if (player.opening_kills > player.opening_deaths) tone = 'win';
      if (player.opening_kills < player.opening_deaths) tone = 'loss';
      return {
        key: player.steamid64 ?? `slot-${player.slot}`,
        name: player.name === '' ? `Jugador ${player.slot}` : player.name,
        text: `${player.opening_kills}-${player.opening_deaths}`,
        tone,
      };
    });
}

/** The opening-duel rate tone; an empty sample reads as neither. */
export function rateTone(rate: TacticalRate): OutcomeTone {
  return rate.total === 0 ? 'even' : winTone(rate);
}
