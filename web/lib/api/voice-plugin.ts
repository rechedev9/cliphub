import { jsonInit, request } from './library.ts';

/**
 * Browser-side contract of the TeamSpeak plugin (`internal/voiceplugin` in Go): the recordings it
 * leaves in Studio's inbox and the CS2 game state config that starts it on its own.
 */

export type RecordingStatus = 'pending' | 'attached';
export type GameStateConfig = 'missing' | 'stale' | 'installed';

export type RecordingTrack = {
  file: string;
  nickname: string;
  uid?: string;
  self?: boolean;
  duration_ms: number;
  /** The team voice track it became in the attached match. */
  match_track?: number;
  /** Why Studio could not read this track; the other tracks still attach. */
  failed?: string;
};

export type VoiceRecording = {
  id: string;
  status: RecordingStatus;
  started_at: string;
  ended_at: string;
  duration_ms: number;
  map?: string;
  server?: string;
  channel?: string;
  trigger: 'auto' | 'manual';
  score_ct: number;
  score_t: number;
  interrupted?: boolean;
  tracks: RecordingTrack[];
  match_id?: string;
  note?: string;
};

export type VoicePluginStatus = {
  inbox: string;
  cs2_found: boolean;
  game_state: GameStateConfig;
  config_path?: string;
  live: { live: boolean; map?: string; score_ct: number; score_t: number; rounds: { round: number }[] };
  pending: number;
};

export const RECORDING_LIST_KEYS = ['recordings'] as const;
export const RECORDING_KEYS = [
  'id', 'status', 'started_at', 'ended_at', 'duration_ms', 'map', 'server', 'channel', 'trigger',
  'score_ct', 'score_t', 'interrupted', 'tracks', 'match_id', 'note',
] as const satisfies readonly (keyof VoiceRecording)[];
export const RECORDING_ATTACH_FIELDS = ['match_id'] as const;
export const VOICE_SPEAKERS_FIELDS = ['speakers'] as const;
export const VOICE_PLUGIN_KEYS = [
  'inbox', 'cs2_found', 'game_state', 'config_path', 'live', 'pending',
] as const satisfies readonly (keyof VoicePluginStatus)[];

export function listVoiceRecordings(): Promise<{ recordings: VoiceRecording[] }> {
  return request('/voice/recordings');
}

export function attachVoiceRecording(target: { id: string; matchId: string }): Promise<VoiceRecording> {
  return request<VoiceRecording>(`/voice/recordings/${encodeURIComponent(target.id)}/attach`, jsonInit('POST', { match_id: target.matchId }));
}

export function deleteVoiceRecording(id: string): Promise<void> {
  return request<void>(`/voice/recordings/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export function getVoicePlugin(): Promise<VoicePluginStatus> {
  return request<VoicePluginStatus>('/voice/plugin');
}

/** Writes CS2's game state config with the current port; CS2 reads it at its next start. */
export function setupVoicePlugin(): Promise<VoicePluginStatus> {
  return request<VoicePluginStatus>('/voice/plugin/setup', { method: 'POST' });
}
