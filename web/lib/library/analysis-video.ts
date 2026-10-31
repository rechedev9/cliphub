import type { LibraryMatch, LibraryMatchPlayer } from '../api/library.ts';
import type { ScoutEvidence } from '../api/scout.ts';
import type { Play } from '../api/types.ts';
import { MAX_SELECTED_ROUNDS, PRODUCE_FORMAT, roundsProduceHref, type ProduceFormat } from '../clips/routes.ts';

/**
 * From a finding to a video. A round, a play or a report row names rounds by
 * match; each video is made from one match, with one player's POV, by the
 * existing Crear flows: the long video (Full Demo) limited to those rounds,
 * or a Short over that player's plays in them.
 */

export const ANALYSIS_VIDEO_ACTIONS = ['pov', 'review', 'short'] as const;
export type AnalysisVideoAction = (typeof ANALYSIS_VIDEO_ACTIONS)[number];

export const ANALYSIS_VIDEO_ACTION: Record<
  AnalysisVideoAction,
  { readonly label: string; readonly format: ProduceFormat; readonly hint: string }
> = {
  pov: { label: 'Grabar POV', format: PRODUCE_FORMAT.full, hint: 'Una ronda entera con la mira del jugador, en 16:9.' },
  review: { label: 'Crear vídeo de review', format: PRODUCE_FORMAT.full, hint: 'Las rondas elegidas, una tras otra, en un vídeo 16:9.' },
  short: { label: 'Enviar a Short', format: PRODUCE_FORMAT.short, hint: 'Las bajas del jugador en esas rondas, en vertical.' },
};

/** One match a finding names, with its rounds in order. */
export type VideoSource = { readonly matchId: string; readonly rounds: readonly number[] };

/** Groups evidence by match, in order of first appearance, with sorted unique rounds. */
export function groupEvidenceByMatch(evidence: readonly Pick<ScoutEvidence, 'match_id' | 'round'>[]): VideoSource[] {
  const order: string[] = [];
  const rounds = new Map<string, Set<number>>();
  for (const e of evidence) {
    let set = rounds.get(e.match_id);
    if (set === undefined) {
      set = new Set();
      rounds.set(e.match_id, set);
      order.push(e.match_id);
    }
    set.add(e.round);
  }
  return order.map((matchId) => ({ matchId, rounds: [...(rounds.get(matchId) ?? [])].sort((a, b) => a - b) }));
}

/** Why an action cannot run on a selection, or null when it can. */
export function actionBlocker(action: AnalysisVideoAction, rounds: readonly number[]): string | null {
  if (rounds.length === 0) return 'Elige al menos una ronda.';
  if (action === 'pov' && rounds.length !== 1) return 'Un POV es de una sola ronda.';
  if (rounds.length > MAX_SELECTED_ROUNDS) return `Un vídeo admite hasta ${MAX_SELECTED_ROUNDS} rondas.`;
  return null;
}

/** Where an action continues once its job exists: the flow's producer with the rounds. */
export function analysisVideoHref(action: AnalysisVideoAction, jobId: string, rounds: readonly number[]): string {
  return roundsProduceHref(jobId, ANALYSIS_VIDEO_ACTION[action].format, rounds);
}

/** The match's players, the given team's first, each team by kills. */
export function videoPlayers(match: LibraryMatch, teamId?: string): LibraryMatchPlayer[] {
  const own = match.teams.find((team) => teamId !== undefined && team.team_id === teamId)?.key;
  return [...(match.players ?? [])].sort((a, b) => {
    const aOwn = a.team_key === own ? 0 : 1;
    const bOwn = b.team_key === own ? 0 : 1;
    return aOwn - bOwn || a.team_key.localeCompare(b.team_key) || b.kills - a.kills || a.steamid64.localeCompare(b.steamid64);
  });
}

/** The POV a video opens with: the asked player when they played, else the top fragger of the asked team. */
export function defaultVideoPlayer(match: LibraryMatch, options: { teamId?: string; steamId?: string }): string | null {
  const players = videoPlayers(match, options.teamId);
  if (options.steamId !== undefined && players.some((p) => p.steamid64 === options.steamId)) return options.steamId;
  return players[0]?.steamid64 ?? null;
}

/** The plays a Short opens with when it comes from analysis: every play of the chosen rounds. */
export function playsInRounds(plays: readonly Pick<Play, 'id' | 'round'>[], rounds: readonly number[]): string[] {
  const wanted = new Set(rounds);
  return plays.filter((play) => wanted.has(play.round)).map((play) => play.id);
}

/** `3, 7 y 12`, the way a round list reads in a sentence. */
export function roundListLabel(rounds: readonly number[]): string {
  const sorted = [...rounds].sort((a, b) => a - b).map(String);
  if (sorted.length <= 1) return sorted.join('');
  return `${sorted.slice(0, -1).join(', ')} y ${sorted[sorted.length - 1]}`;
}
