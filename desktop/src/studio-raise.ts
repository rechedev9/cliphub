// Brings the Studio window forward: on a second launch, and on an assistant's open_in_studio request.

import * as fs from 'node:fs';
import * as path from 'node:path';

// zv-mcp writes the request in the profile and waits for open-ack-<id>.json, written once the page opened or failed.
export const OPEN_REQUEST_FILE = 'open-request.json';
export const OPEN_PATH_CHANNEL = 'cliphub:open-path';
export const OPEN_PATH_ACK_CHANNEL = 'cliphub:open-path-ack';
const REQUEST_VERSION = 2;
const MAX_REQUEST_BYTES = 64 * 1024;
const MAX_PATH_LENGTH = 16 * 1024;
const REQUEST_ID = /^[0-9a-f]{32}$/;
const IN_APP_TIMEOUT_MS = 1500;

export interface RaisableWindow {
  isMinimized(): boolean;
  restore(): void;
  show(): void;
  focus(): void;
  loadURL(url: string): Promise<void>;
  webContents: { getURL(): string; send(channel: string, ...args: unknown[]): void };
}

export interface OpenRequest {
  id: string;
  path: string;
}

export interface OpenRequestOptions {
  userData: string;
  window: () => RaisableWindow | null;
  // The UI origin while the window shows Studio and the assistant may drive it, else null.
  webOrigin: () => string | null;
  log: (text: string) => void;
  // Routes the path inside the loaded app; true once the app took it.
  routeInApp: (request: OpenRequest) => Promise<boolean>;
}

export function bringForward(win: RaisableWindow | null): void {
  if (win === null) return;
  if (win.isMinimized()) win.restore();
  win.show();
  win.focus();
}

// isStudioPagePath keeps app pages only: no API route, Next asset or file. web/lib/desktop-navigation.ts checks the same.
export function isStudioPagePath(target: string): boolean {
  if (target.length > MAX_PATH_LENGTH || !target.startsWith('/') || target.startsWith('//') || /[\\\r\n]/.test(target)) return false;
  const pathname = target.split(/[?#]/, 1)[0] ?? '';
  if (pathname === '/api' || pathname.startsWith('/api/') || pathname.startsWith('/_next/')) return false;
  return !(pathname.split('/').pop() ?? '').includes('.');
}

// parseOpenRequest reads the request's id and path; null when it is not a version 2 request.
export function parseOpenRequest(raw: string): { id: string; path: unknown } | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null || !('version' in parsed) || !('id' in parsed) || !('path' in parsed)) return null;
  const { version, id } = parsed;
  if (version !== REQUEST_VERSION || typeof id !== 'string' || !REQUEST_ID.test(id)) return null;
  return { id, path: parsed.path };
}

// readRequestFile takes the pending request off disk: a regular file of at most 64 KB, else nothing.
function readRequestFile(file: string, log: (text: string) => void): string | null {
  let stat: fs.Stats;
  try {
    stat = fs.lstatSync(file);
  } catch {
    return null;
  }
  if (stat.isDirectory()) {
    log('[open] ignored a folder in place of an assistant request');
    return null;
  }
  try {
    if (!stat.isFile() || stat.size > MAX_REQUEST_BYTES) {
      log('[open] dropped an assistant request that is not a small regular file');
      return null;
    }
    return fs.readFileSync(file, 'utf8');
  } catch {
    return null;
  } finally {
    fs.rmSync(file, { force: true });
  }
}

function writeAck(options: { userData: string; id: string; ok: boolean; reason?: string }): void {
  const file = path.join(options.userData, `open-ack-${options.id}.json`);
  const tmp = `${file}.tmp`;
  fs.writeFileSync(tmp, JSON.stringify({ version: REQUEST_VERSION, id: options.id, ok: options.ok, reason: options.reason ?? '' }));
  fs.renameSync(tmp, file);
}

async function openPage(options: OpenRequestOptions, request: OpenRequest): Promise<{ ok: boolean; reason?: string }> {
  const win = options.window();
  const origin = options.webOrigin();
  if (win === null || origin === null) return { ok: false, reason: 'Studio is still loading, or assistant access is off' };
  if (!isStudioPagePath(request.path)) return { ok: false, reason: 'not a Studio page path' };
  const url = new URL(request.path, origin);
  if (url.origin !== origin) return { ok: false, reason: 'not a Studio page path' };
  bringForward(win);
  let onStudio = false;
  try {
    onStudio = new URL(win.webContents.getURL()).origin === origin;
  } catch {
    onStudio = false;
  }
  if (onStudio && (await options.routeInApp(request))) return { ok: true };
  try {
    await win.loadURL(url.href);
    return { ok: true };
  } catch (err) {
    options.log(`[open] could not open ${url.pathname}: ${String(err)}`);
    return { ok: false, reason: 'the page did not load' };
  }
}

// handleOpenRequest takes the pending request, opens its page and acks the outcome.
export async function handleOpenRequest(options: OpenRequestOptions): Promise<void> {
  const raw = readRequestFile(path.join(options.userData, OPEN_REQUEST_FILE), options.log);
  if (raw === null) return;
  const parsed = parseOpenRequest(raw);
  if (parsed === null) {
    options.log('[open] ignored a malformed assistant request');
    return;
  }
  const result = typeof parsed.path === 'string'
    ? await openPage(options, { id: parsed.id, path: parsed.path })
    : { ok: false, reason: 'not a Studio page path' };
  try {
    writeAck({ userData: options.userData, id: parsed.id, ...result });
  } catch (err) {
    options.log(`[open] could not answer the assistant: ${String(err)}`);
  }
}

interface AckChannel {
  on(channel: string, listener: (event: unknown, id: unknown) => void): unknown;
}

// ipcRouter sends the path to the app's router and resolves true when the app acks it in time.
export function ipcRouter(options: { ipc: AckChannel; window: () => RaisableWindow | null; timeoutMs?: number }): (request: OpenRequest) => Promise<boolean> {
  const pending = new Map<string, (taken: boolean) => void>();
  options.ipc.on(OPEN_PATH_ACK_CHANNEL, (_event, id) => {
    if (typeof id !== 'string') return;
    pending.get(id)?.(true);
    pending.delete(id);
  });
  return (request) => new Promise((resolve) => {
    const win = options.window();
    if (win === null) {
      resolve(false);
      return;
    }
    const timer = setTimeout(() => {
      pending.delete(request.id);
      resolve(false);
    }, options.timeoutMs ?? IN_APP_TIMEOUT_MS);
    pending.set(request.id, (taken) => {
      clearTimeout(timer);
      resolve(taken);
    });
    win.webContents.send(OPEN_PATH_CHANNEL, request);
  });
}

export function watchOpenRequests(options: OpenRequestOptions): fs.FSWatcher | null {
  try {
    const watcher = fs.watch(options.userData, (_event, name) => {
      if (name === OPEN_REQUEST_FILE) void handleOpenRequest(options);
    });
    watcher.on('error', (err) => options.log(`[open] stopped watching for assistant requests: ${String(err)}`));
    watcher.unref();
    return watcher;
  } catch (err) {
    options.log(`[open] cannot watch for assistant requests: ${String(err)}`);
    return null;
  }
}
