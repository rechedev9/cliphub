/**
 * What the web proxy's route for the orchestrator's replay-video endpoint
 * (`internal/httpapi/library_replay_video.go`) accepts and answers with.
 */
export type ReplayVideoResult = { video_id: string; bytes: number; frames: number; seconds: number; voice: boolean };

export const REPLAY_VIDEO_KEYS = ['video_id', 'bytes', 'frames', 'seconds', 'voice'] as const satisfies readonly (keyof ReplayVideoResult)[];
/** One request, as the orchestrator caps it: 120 s of 60 fps frames and the spec. */
export const REPLAY_VIDEO_BYTES = 3 * 1024 * 1024 * 1024;
