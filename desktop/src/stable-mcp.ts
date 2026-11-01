import * as fs from 'node:fs';
import * as path from 'node:path';

export interface McpFileStamp {
  size: number;
  mtimeMs: number;
}

export interface SyncStableMcpOptions {
  /** The zv-mcp bundled with the install. */
  source: string;
  /** Where assistants launch it from, outside the install folder. */
  target: string;
  renameFile?: (from: string, to: string) => void;
}

export interface StableMcp {
  /** The path to hand to an assistant. */
  path: string;
  /** What to log when the copy did not go as planned, else null. */
  problem: string | null;
}

// Coarse enough to survive file systems that round modification times.
const MTIME_TOLERANCE_MS = 2_000;

/**
 * Assistants keep zv-mcp running, and a running executable in the install folder
 * blocks the updater. They launch a copy under userData instead.
 */
export function mcpCopyNeeded({ source, target }: { source: McpFileStamp; target: McpFileStamp | null }): boolean {
  return target === null
    || target.size !== source.size
    || Math.abs(target.mtimeMs - source.mtimeMs) >= MTIME_TOLERANCE_MS;
}

/** Brings the stable copy up to date. Never throws; an in-use copy keeps its old content. */
export function syncStableMcp({ source, target, renameFile = fs.renameSync }: SyncStableMcpOptions): StableMcp {
  const stagedCopy = `${target}.new`;
  try {
    const sourceStamp = stampOf(source);
    if (!mcpCopyNeeded({ source: sourceStamp, target: statOrNull(target) })) return { path: target, problem: null };
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(source, stagedCopy);
    fs.utimesSync(stagedCopy, sourceStamp.mtimeMs / 1000, sourceStamp.mtimeMs / 1000);
    renameFile(stagedCopy, target);
    return { path: target, problem: null };
  } catch (error) {
    removeQuietly(stagedCopy);
    const problem = `could not update ${target}: ${String(error)}`;
    // An older copy still works; only a missing one sends the assistant to the install folder.
    return { path: statOrNull(target) === null ? source : target, problem };
  }
}

function stampOf(file: string): McpFileStamp {
  const { size, mtimeMs } = fs.statSync(file);
  return { size, mtimeMs };
}

function statOrNull(file: string): McpFileStamp | null {
  try {
    return stampOf(file);
  } catch {
    return null;
  }
}

function removeQuietly(file: string): void {
  try {
    fs.rmSync(file, { force: true });
  } catch {
    // A leftover staged copy is overwritten by the next attempt.
  }
}
