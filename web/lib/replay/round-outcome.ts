/** How a round ended, as the strip draws it. */
export type RoundOutcome = 'exploded' | 'defused' | 'elimination' | 'time' | 'other';

export const ROUND_OUTCOME_LABEL: Record<RoundOutcome, string> = {
  exploded: 'Bomba explotada',
  defused: 'Bomba desactivada',
  elimination: 'Eliminación',
  time: 'Tiempo agotado',
  other: 'Fin de ronda',
};

/** The tactical document's `end_reason` slug (`tactical.endReasonSlug`) as an outcome. */
export function roundOutcome(endReason: string): RoundOutcome {
  switch (endReason) {
    case 'bomb_exploded':
      return 'exploded';
    case 'bomb_defused':
      return 'defused';
    case 'ct_eliminated_t':
    case 't_eliminated_ct':
      return 'elimination';
    case 'time_expired':
      return 'time';
    default:
      return 'other';
  }
}
