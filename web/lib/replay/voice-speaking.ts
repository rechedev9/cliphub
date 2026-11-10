import type { VoiceSpan } from '../api/voice.ts';

/** A team voice track tied to a player, with where its audio has voice (audio time, sorted). */
export type SpeakingTrack = { track: number; steamid64?: string; speech: readonly VoiceSpan[] };

/** How long a speaker stays marked past the end of a span, so short gaps between words do not flicker. */
const HOLD_MS = 250;

/** The players heard at an audio time: tracks tied to a player, not muted, inside a speech span. */
export function speakingSteamIds(input: { tracks: readonly SpeakingTrack[]; muted: ReadonlySet<number>; audioMs: number }): Set<string> {
  const out = new Set<string>();
  for (const track of input.tracks) {
    if (!track.steamid64 || input.muted.has(track.track)) continue;
    if (inSpeech(track.speech, input.audioMs)) out.add(track.steamid64);
  }
  return out;
}

function inSpeech(spans: readonly VoiceSpan[], at: number): boolean {
  let lo = 0;
  let hi = spans.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    const span = spans[mid];
    if (span === undefined) return false;
    if (at < span.start_ms) hi = mid - 1;
    else if (at > span.end_ms + HOLD_MS) lo = mid + 1;
    else return true;
  }
  return false;
}
