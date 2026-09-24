import { randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';
import type { DiagnosticLogInput } from './diagnostic-log.ts';

// Crash signals that survive the process that crashed: exit classification for
// backend children, the previous-session marker checked against local Crashpad
// minidumps, and the Go runtime's crash output file. Minidumps never leave the
// machine; only their existence is reported.

/** DBG_TERMINATE_PROCESS: Windows ends every process this way on log off or shutdown. */
export const EXIT_SESSION_TERMINATED = 1073807364;
/** STATUS_DLL_INIT_FAILED: a child started while the desktop session was closing. */
export const EXIT_DLL_INIT_FAILED = 3221225794;

export type ExitClass = 'shutdown_kill' | 'crash';

export function classifyExit(code: number | null, shuttingDown: boolean): ExitClass {
  if (code === EXIT_SESSION_TERMINATED) return 'shutdown_kill';
  if (code === EXIT_DLL_INIT_FAILED && shuttingDown) return 'shutdown_kill';
  return 'crash';
}

interface SessionMarker {
  version: 1;
  startedAt: string;
  electron: string;
  minidumps: number;
  shuttingDown: boolean;
}

export interface PreviousSessionOptions {
  markerPath: string;
  crashDumpsDir: string;
  electronVersion: string;
  record: (input: DiagnosticLogInput) => void;
  log: (text: string) => void;
  now?: () => Date;
}

export interface PreviousSessionResult {
  crashed: boolean;
  minidump: boolean;
}

/**
 * Reports a previous session that never reached a clean quit, then marks this
 * one as running. The marker is removed on clean quit, and flagged instead when
 * Windows ends the session, so a log off is not reported as a crash.
 */
export function recordPreviousSession(options: PreviousSessionOptions): PreviousSessionResult {
  const previous = readMarker(options.markerPath);
  const dumps = countMinidumps(options.crashDumpsDir);
  const result: PreviousSessionResult = { crashed: false, minidump: false };
  if (previous !== null) {
    result.minidump = dumps > previous.minidumps;
    if (previous.shuttingDown) {
      options.log('[runtime] previous session ended with the Windows session\n');
    } else {
      result.crashed = true;
      options.log(`[runtime] previous session did not quit cleanly (minidump=${result.minidump ? 'yes' : 'no'})\n`);
      // The main process is the only one whose death skips the clean-quit path;
      // renderer, GPU and utility crashes are reported live.
      options.record({
        event: 'session.crashed_previous',
        level: 'error',
        message: `process_type=browser minidump=${result.minidump ? 'yes' : 'no'} electron=${previous.electron === 'unknown' ? 'unknown' : `v${previous.electron}`}`,
      });
    }
  }
  try {
    writeMarker(options.markerPath, {
      version: 1,
      startedAt: (options.now?.() ?? new Date()).toISOString(),
      electron: safeVersion(options.electronVersion),
      minidumps: dumps,
      shuttingDown: false,
    });
  } catch (error) {
    options.log(`[runtime] could not write the session marker: ${String(error)}\n`);
  }
  return result;
}

/** Keeps the marker but records that Windows is ending the session. */
export function markSessionShuttingDown(markerPath: string): void {
  const marker = readMarker(markerPath);
  if (marker === null || marker.shuttingDown) return;
  try {
    writeMarker(markerPath, { ...marker, shuttingDown: true });
  } catch {
    // Best effort: the next start then reports this session as a crash.
  }
}

export function clearSessionMarker(markerPath: string): void {
  try {
    fs.rmSync(markerPath, { force: true });
  } catch {
    // A leftover marker only produces one extra previous-session report.
  }
}

/** Crashpad keeps reports as .dmp files directly under the dir or in reports/, pending/, completed/. */
export function countMinidumps(crashDumpsDir: string): number {
  let count = 0;
  const visit = (dir: string, depth: number): void => {
    let entries: fs.Dirent[];
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (entry.isFile() && entry.name.toLowerCase().endsWith('.dmp')) count++;
      else if (entry.isDirectory() && depth > 0) visit(path.join(dir, entry.name), depth - 1);
    }
  };
  visit(crashDumpsDir, 1);
  return count;
}

const CRASH_HEAD_LINES = 30;
const CRASH_TAIL_LINES = 150;

/**
 * Reads and deletes the orchestrator's Go crash output (ZV_CRASH_OUTPUT). The
 * head keeps the panic line and the tail the last frames when the dump is long.
 */
export function takeCrashOutput(file: string): string | null {
  let text: string;
  try {
    text = fs.readFileSync(file, 'utf8');
  } catch {
    return null;
  }
  try {
    fs.rmSync(file, { force: true });
  } catch {
    // Still report it; a stale file is reported again rather than lost.
  }
  const lines = text.replace(/\r\n/g, '\n').split('\n');
  while (lines.length > 0 && lines[lines.length - 1]?.trim() === '') lines.pop();
  if (lines.every((line) => line.trim() === '')) return null;
  if (lines.length <= CRASH_HEAD_LINES + CRASH_TAIL_LINES) return lines.join('\n');
  const omitted = lines.length - CRASH_HEAD_LINES - CRASH_TAIL_LINES;
  return [...lines.slice(0, CRASH_HEAD_LINES), `[${omitted} lines omitted]`, ...lines.slice(-CRASH_TAIL_LINES)].join('\n');
}

function readMarker(markerPath: string): SessionMarker | null {
  let value: unknown;
  try {
    value = JSON.parse(fs.readFileSync(markerPath, 'utf8'));
  } catch (error) {
    // A marker that exists but cannot be parsed still means an unclean exit.
    if (isRecord(error) && error.code === 'ENOENT') return null;
    return fs.existsSync(markerPath) ? { version: 1, startedAt: '', electron: 'unknown', minidumps: 0, shuttingDown: false } : null;
  }
  if (!isRecord(value)) return { version: 1, startedAt: '', electron: 'unknown', minidumps: 0, shuttingDown: false };
  return {
    version: 1,
    startedAt: typeof value.startedAt === 'string' ? value.startedAt : '',
    electron: typeof value.electron === 'string' ? safeVersion(value.electron) : 'unknown',
    minidumps: typeof value.minidumps === 'number' && Number.isSafeInteger(value.minidumps) && value.minidumps >= 0 ? value.minidumps : 0,
    shuttingDown: value.shuttingDown === true,
  };
}

function writeMarker(markerPath: string, marker: SessionMarker): void {
  fs.mkdirSync(path.dirname(markerPath), { recursive: true });
  const temporary = `${markerPath}.${randomUUID()}.tmp`;
  fs.writeFileSync(temporary, `${JSON.stringify(marker)}\n`, { encoding: 'utf8', mode: 0o600 });
  try {
    fs.renameSync(temporary, markerPath);
  } catch (error) {
    fs.rmSync(temporary, { force: true });
    throw error;
  }
}

function safeVersion(value: string): string {
  return /^\d{1,5}(?:\.\d{1,5}){0,3}$/.test(value) ? value : 'unknown';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
