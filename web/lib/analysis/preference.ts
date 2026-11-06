import type { MapPreference } from '../api/veto.ts';

/** How a team's map preference (`library.MapPreference`) reads on a card or in the veto table. */

/** Scores this far from zero read as a clear lean. */
const CLEAR_LEAN = 30;

/** "+42", "-15" or "0". */
export function preferenceScoreText(score: number): string {
  if (score > 0) return `+${score}`;
  if (score < 0) return `-${Math.abs(score)}`;
  return '0';
}

/** "La busca", "La evita" or "Sin preferencia clara". */
export function preferenceWord(score: number): string {
  if (score >= CLEAR_LEAN) return 'La busca';
  if (score <= -CLEAR_LEAN) return 'La evita';
  return 'Sin preferencia clara';
}

function part(value: number): string {
  return preferenceScoreText(Math.round(value * 100));
}

/** The tooltip: what the score is made of, with this map's parts and samples. */
export function preferenceExplanation(preference: MapPreference): string {
  const vetoes = preference.vetoes === 1 ? '1 pick o ban' : `${preference.vetoes} picks y bans`;
  const matches = preference.matches === 1 ? '1 partida' : `${preference.matches} partidas`;
  return (
    `De -100 (siempre la evita) a +100 (siempre la busca). Pesa la mitad el veto (${part(preference.veto)}, ${vetoes}), ` +
    `un cuarto lo que la juega frente a su mapa más jugado (${part(preference.play)}) ` +
    `y un cuarto lo que gana en ella (${part(preference.win)}, ${matches}). Con pocos datos se queda cerca de 0.`
  );
}
