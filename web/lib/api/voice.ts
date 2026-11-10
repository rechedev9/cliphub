import { jsonInit, request } from './library.ts';

/**
 * Browser-side contract of a match's team voice (`library.MatchVoice` in Go):
 * TeamSpeak tracks placed on the demo by an offset, and their local transcript.
 */

export type VoiceSyncState = 'unsynced' | 'auto' | 'manual';
export type VoiceTranscriptState = 'none' | 'queued' | 'running' | 'ready' | 'failed';
export type TranscriberState = 'not_installed' | 'installing' | 'installed' | 'failed' | 'unsupported';
export type VoicePhase = 'freezetime' | 'midround' | 'post_plant' | 'dead' | 'after_round';

/** The server's caps (`library.MaxVoice*`). */
export const MAX_VOICE_LABEL = 60;
export const MAX_VOICE_OFFSET_MS = 6 * 3600 * 1000;

export type VoiceTrack = {
  track: number;
  label: string;
  steamid64?: string;
  file_name: string;
  channel: number;
  duration_ms: number;
  size_bytes: number;
  source: string;
};

/** Audio time 0 plays at demo time `offset_ms`. */
export type VoiceSync = {
  state: VoiceSyncState;
  method: string;
  offset_ms: number;
  auto_offset_ms: number;
  confidence: number;
  note?: string;
  team_key?: string;
};

export type VoiceTranscript = { state: VoiceTranscriptState; language?: string; model?: string; error?: string; progress?: number };

export type VoiceTranscriber = {
  state: TranscriberState;
  version: string;
  model: string;
  download_bytes: number;
  done_bytes?: number;
  error?: string;
};

export type MatchVoice = {
  match_id: string;
  present: boolean;
  tracks: VoiceTrack[];
  duration_ms: number;
  speech_ms: number;
  sync: VoiceSync;
  transcript: VoiceTranscript;
  transcriber: VoiceTranscriber;
};

export type VoiceImportFile = { file_name: string; status: string; tracks: number; error_code?: string; error?: string };

export type VoicePhaseTalk = {
  freezetime_ms: number;
  midround_ms: number;
  post_plant_ms: number;
  dead_ms: number;
  after_round_ms: number;
};

export type VoiceSpeakerTalk = VoicePhaseTalk & { track: number; label: string; steamid64?: string };

export type VoiceRoundTalk = { round: number; speakers: VoiceSpeakerTalk[] };

export type VoiceSpan = { start_ms: number; end_ms: number };

export type VoiceCommsLine = {
  track: number;
  label: string;
  steamid64?: string;
  start_ms: number;
  end_ms: number;
  tick: number;
  round: number;
  round_ms: number;
  phase?: VoicePhase;
  text: string;
};

export type VoiceComms = {
  match_id: string;
  sync: VoiceSync;
  transcript: VoiceTranscript;
  tracks: { track: number; label: string; steamid64?: string; speech: VoiceSpan[] }[];
  rounds: VoiceRoundTalk[];
  totals: VoiceSpeakerTalk[];
  lines: VoiceCommsLine[];
};

export type VoiceSearchHit = VoiceCommsLine & { match_id: string; map: string; played_at: string; teams: string[] };

export const VOICE_KEYS = [
  'match_id', 'present', 'tracks', 'duration_ms', 'speech_ms', 'sync', 'transcript', 'transcriber',
] as const satisfies readonly (keyof MatchVoice)[];
export const VOICE_IMPORT_KEYS = [...VOICE_KEYS, 'files'] as const;
export const VOICE_SYNC_FIELDS = ['offset_ms', 'auto'] as const;
export const VOICE_TRACK_FIELDS = ['label', 'steamid64'] as const;
export const VOICE_TRANSCRIBE_FIELDS = ['language'] as const;
export const VOICE_COMMS_KEYS = [
  'match_id', 'sync', 'transcript', 'tracks', 'rounds', 'totals', 'lines',
] as const satisfies readonly (keyof VoiceComms)[];
export const VOICE_COMMS_QUERY = ['round'] as const;
export const VOICE_SEARCH_QUERY = ['q', 'match_id', 'team_id', 'from', 'to', 'limit'] as const;
export const VOICE_SEARCH_KEYS = ['hits', 'truncated'] as const;
export const TRANSCRIBER_KEYS = [
  'state', 'version', 'model', 'download_bytes', 'done_bytes', 'error',
] as const satisfies readonly (keyof VoiceTranscriber)[];
/** One import request: a two hour TeamSpeak WAV with room to spare, as the orchestrator reads. */
export const VOICE_UPLOAD_BYTES = 2 * 1024 * 1024 * 1024;

function voicePath(matchId: string): string {
  return `/matches/${encodeURIComponent(matchId)}/voice`;
}

/** Where the Replay's audio element reads a track, with ranges. */
export function voiceTrackAudioUrl(matchId: string, track: number): string {
  return `/api/library${voicePath(matchId)}/tracks/${track}/audio`;
}

export function getMatchVoice(matchId: string): Promise<MatchVoice> {
  return request<MatchVoice>(voicePath(matchId));
}

/** Uploads TeamSpeak files; the first import of a match places them on its own. */
export function importMatchVoice(matchId: string, files: readonly File[]): Promise<MatchVoice & { files: VoiceImportFile[] }> {
  const form = new FormData();
  for (const file of files) form.append('audio', file, file.name);
  return request(voicePath(matchId), { method: 'POST', body: form });
}

export function setVoiceOffset(matchId: string, offsetMs: number): Promise<MatchVoice> {
  return request<MatchVoice>(voicePath(matchId), jsonInit('PATCH', { offset_ms: Math.round(offsetMs) }));
}

export function autoSyncVoice(matchId: string): Promise<MatchVoice> {
  return request<MatchVoice>(voicePath(matchId), jsonInit('PATCH', { auto: true }));
}

export function patchVoiceTrack(target: { matchId: string; track: number }, patch: { label?: string; steamid64?: string }): Promise<MatchVoice> {
  return request<MatchVoice>(`${voicePath(target.matchId)}/tracks/${target.track}`, jsonInit('PATCH', patch));
}

export function deleteMatchVoice(matchId: string): Promise<void> {
  return request<void>(voicePath(matchId), { method: 'DELETE' });
}

export function transcribeMatchVoice(matchId: string, language: string): Promise<MatchVoice> {
  return request<MatchVoice>(`${voicePath(matchId)}/transcribe`, jsonInit('POST', { language }));
}

export function getVoiceComms(matchId: string): Promise<VoiceComms> {
  return request<VoiceComms>(`${voicePath(matchId)}/comms`);
}

export function searchVoice(query: { q: string; teamId?: string; limit?: number }): Promise<{ hits: VoiceSearchHit[]; truncated: boolean }> {
  const params = new URLSearchParams({ q: query.q });
  if (query.teamId) params.set('team_id', query.teamId);
  if (query.limit !== undefined) params.set('limit', String(query.limit));
  return request(`/voice/search?${params.toString()}`);
}

export function getTranscriber(): Promise<VoiceTranscriber> {
  return request<VoiceTranscriber>('/voice/transcriber');
}

export function installTranscriber(): Promise<VoiceTranscriber> {
  return request<VoiceTranscriber>('/voice/transcriber/install', { method: 'POST' });
}
