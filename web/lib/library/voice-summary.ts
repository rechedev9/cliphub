import type { VoiceSummary, VoiceSync } from '../api/voice.ts';

export type VoiceByMatch = ReadonlyMap<string, VoiceSummary>;

export function voiceIndex(summaries: readonly VoiceSummary[]): VoiceByMatch {
  return new Map(summaries.map((summary) => [summary.match_id, summary]));
}

/** The sync state of a recording in words, for the tooltip of the voice icon. */
export function voiceSyncTitle(sync: VoiceSync): string {
  if (sync.state === 'manual') return 'Voz sincronizada a mano';
  if (sync.state === 'auto') return `Voz sincronizada sola (confianza ${Math.round(sync.confidence * 100)} %)`;
  return 'Voz sin sincronizar';
}

/** What a row says after Volver a sincronizar voz: the new state, and the reason when the recording still does not fit. */
export function resyncOutcome(sync: VoiceSync): { tone: 'ok' | 'warning'; text: string } {
  if (sync.state === 'unsynced') {
    return { tone: 'warning', text: sync.note ? `Sigue sin sincronizar. ${sync.note}` : 'Sigue sin sincronizar.' };
  }
  return { tone: 'ok', text: `${voiceSyncTitle(sync)}.` };
}
