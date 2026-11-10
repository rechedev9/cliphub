import { randomUUID } from 'node:crypto';
import { safeFileName } from './safe-file-name.ts';

export const STUDIO_REPLAY_GIF_CHANNEL = 'cliphub:replay-gif';

/** The Replay saves a GIF or, since the MP4 export, a video through the same two messages. */
export type ReplayFileFormat = 'gif' | 'mp4';

const MAX_FILE_BYTES = 512 * 1024 * 1024;
const MAX_VIDEO_BYTES = 1024 * 1024 * 1024;
const GIF_SIGNATURE = 'GIF89a';
const MP4_BOX = 'ftyp';
/** A long tramo takes minutes to encode; a chosen path stays valid this long. */
const TARGET_TTL_MS = 30 * 60 * 1000;

export type ReplayGifRequest =
  | { action: 'pick'; fileName: string; format: ReplayFileFormat }
  | { action: 'write'; token: string; bytes: Uint8Array; format: ReplayFileFormat };

/** Parses the Replay's two save messages: ask where to save, then write the encoded file there. */
export function parseReplayGifRequest(value: unknown): ReplayGifRequest {
  if (!isRecord(value)) throw new Error('invalid replay gif request');
  if (value.action === 'pick' && typeof value.fileName === 'string') {
    const keys = Object.keys(value).length;
    let format: ReplayFileFormat = 'gif';
    if (keys === 3 && value.format === 'mp4') format = 'mp4';
    else if (keys !== 2) throw new Error('invalid replay gif request');
    return { action: 'pick', fileName: safeFileName(value.fileName, { extension: format, fallback: 'replay' }), format };
  }
  if (value.action !== 'write' || Object.keys(value).length !== 3) throw new Error('invalid replay gif request');
  const { token, bytes } = value;
  if (typeof token !== 'string' || token === '' || !(bytes instanceof Uint8Array)) throw new Error('invalid replay gif request');
  const format = formatOf(bytes);
  if (format === null) throw new Error('replay file is neither a gif nor an mp4');
  if (bytes.length > (format === 'mp4' ? MAX_VIDEO_BYTES : MAX_FILE_BYTES)) throw new Error('replay file is too large');
  return { action: 'write', token, bytes, format };
}

function startsWith(bytes: Uint8Array, text: { at: number; value: string }): boolean {
  if (bytes.length < text.at + text.value.length) return false;
  return [...text.value].every((char, index) => bytes[text.at + index] === char.charCodeAt(0));
}

function formatOf(bytes: Uint8Array): ReplayFileFormat | null {
  if (startsWith(bytes, { at: 0, value: GIF_SIGNATURE })) return 'gif';
  if (startsWith(bytes, { at: 4, value: MP4_BOX })) return 'mp4';
  return null;
}

export type ReplayTarget = { path: string; format: ReplayFileFormat };

/**
 * The paths the user picked in the save dialog, behind one-use tokens: the
 * renderer never names a path, it only hands back the token it was given.
 */
export class GifTargets {
  private readonly targets = new Map<string, ReplayTarget & { expires: number }>();

  add(target: ReplayTarget, now: number): string {
    for (const [token, entry] of this.targets) {
      if (entry.expires <= now) this.targets.delete(token);
    }
    const token = randomUUID();
    this.targets.set(token, { ...target, expires: now + TARGET_TTL_MS });
    return token;
  }

  take(token: string, now: number): ReplayTarget | null {
    const entry = this.targets.get(token);
    this.targets.delete(token);
    return entry === undefined || entry.expires <= now ? null : { path: entry.path, format: entry.format };
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
