import { LibraryApiError, request } from './library.ts';

/**
 * Browser-side contract of the Replay's MP4 (`internal/httpapi/library_replay_video.go`): a spec, then
 * the frames as JPEGs; the orchestrator encodes them with the team voice and keeps the MP4 for one download.
 */

export type ReplayVideoSegment = { round: number; from_tick: number; to_tick: number; frames: number };

export type ReplayVideoSpec = {
  fps: number;
  width: number;
  height: number;
  speed: number;
  include_voice: boolean;
  /** The tracks heard; every track of the match when empty. */
  voice_tracks: number[];
  segments: ReplayVideoSegment[];
};

export type ReplayVideoResult = { video_id: string; bytes: number; frames: number; seconds: number; voice: boolean };

export const REPLAY_VIDEO_KEYS = ['video_id', 'bytes', 'frames', 'seconds', 'voice'] as const satisfies readonly (keyof ReplayVideoResult)[];
/** One export request, as the orchestrator caps it: 120 s of 60 fps frames and the spec. */
export const REPLAY_VIDEO_BYTES = 3 * 1024 * 1024 * 1024;

const MESSAGES: Readonly<Record<string, string>> = {
  replay_video_unavailable: 'Este Studio no tiene FFmpeg para crear vídeos.',
  voice_unavailable: 'Este Studio no tiene FFmpeg para la voz del equipo.',
  no_voice: 'Esta partida no tiene voz del equipo: impórtala primero.',
  voice_unsynced: 'La voz del equipo aún no está sincronizada con la demo: sincronízala antes de exportar.',
  voice_outside: 'La grabación de voz no cubre este tramo: quita la voz o elige otro tramo.',
  replay_video_failed: 'FFmpeg no pudo crear el MP4. Prueba con un tramo más corto.',
};
const FAILED = 'No se pudo crear el MP4.';

/** The Spanish line for a failed export. */
export function replayVideoMessage(error: unknown): string {
  if (error instanceof LibraryApiError && error.code !== undefined) return MESSAGES[error.code] ?? FAILED;
  return FAILED;
}

/** Sends the spec and the frames in order; answers once the MP4 is encoded. */
export function createReplayVideo(options: { matchId: string; spec: ReplayVideoSpec; frames: readonly Blob[]; signal: AbortSignal }): Promise<ReplayVideoResult> {
  const form = new FormData();
  form.append('spec', JSON.stringify(options.spec));
  for (const frame of options.frames) form.append('frame', frame, 'frame.jpg');
  return request<ReplayVideoResult>(`/matches/${encodeURIComponent(options.matchId)}/replay-video`, { method: 'POST', body: form, signal: options.signal });
}

/** The encoded MP4's bytes; the orchestrator drops it once read. */
export async function readReplayVideo(videoId: string, signal: AbortSignal): Promise<Uint8Array<ArrayBuffer>> {
  const res = await fetch(`/api/library/replay-videos/${encodeURIComponent(videoId)}`, { cache: 'no-store', signal });
  if (!res.ok) throw new LibraryApiError(`replay video download failed (${res.status})`, res.status);
  return new Uint8Array(await res.arrayBuffer());
}
