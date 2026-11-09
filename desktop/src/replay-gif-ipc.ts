import { randomUUID } from 'node:crypto';
import { safeFileName } from './safe-file-name.ts';

export const STUDIO_REPLAY_GIF_CHANNEL = 'cliphub:replay-gif';

const MAX_GIF_BYTES = 512 * 1024 * 1024;
const GIF_SIGNATURE = 'GIF89a';
/** A long tramo takes minutes to encode; a chosen path stays valid this long. */
const TARGET_TTL_MS = 30 * 60 * 1000;

export type ReplayGifRequest =
  | { action: 'pick'; fileName: string }
  | { action: 'write'; token: string; bytes: Uint8Array };

/** Parses the Replay's two GIF messages: ask where to save, then write the encoded GIF there. */
export function parseReplayGifRequest(value: unknown): ReplayGifRequest {
  if (!isRecord(value)) throw new Error('invalid replay gif request');
  if (value.action === 'pick' && Object.keys(value).length === 2 && typeof value.fileName === 'string') {
    return { action: 'pick', fileName: safeFileName(value.fileName, { extension: 'gif', fallback: 'replay' }) };
  }
  if (value.action !== 'write' || Object.keys(value).length !== 3) throw new Error('invalid replay gif request');
  const { token, bytes } = value;
  if (typeof token !== 'string' || token === '' || !(bytes instanceof Uint8Array)) throw new Error('invalid replay gif request');
  if (bytes.length > MAX_GIF_BYTES) throw new Error('replay gif is too large');
  if (!isGif(bytes)) throw new Error('replay gif is not a gif');
  return { action: 'write', token, bytes };
}

function isGif(bytes: Uint8Array): boolean {
  if (bytes.length < GIF_SIGNATURE.length) return false;
  return [...GIF_SIGNATURE].every((char, index) => bytes[index] === char.charCodeAt(0));
}

/**
 * The paths the user picked in the save dialog, behind one-use tokens: the
 * renderer never names a path, it only hands back the token it was given.
 */
export class GifTargets {
  private readonly targets = new Map<string, { path: string; expires: number }>();

  add(path: string, now: number): string {
    for (const [token, target] of this.targets) {
      if (target.expires <= now) this.targets.delete(token);
    }
    const token = randomUUID();
    this.targets.set(token, { path, expires: now + TARGET_TTL_MS });
    return token;
  }

  take(token: string, now: number): string | null {
    const target = this.targets.get(token);
    this.targets.delete(token);
    return target === undefined || target.expires <= now ? null : target.path;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
