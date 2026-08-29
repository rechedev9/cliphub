import { timingSafeEqual } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { downloadFile, fetchText, type DownloadOptions } from './http-download.ts';

export const GITHUB_LATEST_RELEASE_URL =
  'https://api.github.com/repos/rechedev9/cliphub/releases/latest';

const CHECKSUM_LINE = /^([a-f0-9]{64})  ([^\r\n]+)$/;
const VERSION_PATTERN = /^v?(\d+)\.(\d+)\.(\d+)$/;
const PROGRESS_THROTTLE_MS = 200;

export const APP_UPDATE_STATE = {
  unavailable: 'unavailable',
  idle: 'idle',
  checking: 'checking',
  current: 'current',
  available: 'available',
  downloading: 'downloading',
  ready: 'ready',
  installing: 'installing',
  error: 'error',
} as const;

export type AppUpdateStatus =
  | { state: typeof APP_UPDATE_STATE.unavailable }
  | { state: typeof APP_UPDATE_STATE.idle }
  | { state: typeof APP_UPDATE_STATE.checking }
  | { state: typeof APP_UPDATE_STATE.current; version: string }
  | { state: typeof APP_UPDATE_STATE.available; version: string; currentVersion: string }
  | {
    state: typeof APP_UPDATE_STATE.downloading;
    version: string;
    received: number;
    total: number | null;
  }
  | { state: typeof APP_UPDATE_STATE.ready; version: string }
  | { state: typeof APP_UPDATE_STATE.installing; version: string }
  | { state: typeof APP_UPDATE_STATE.error; message: string };

export interface AppUpdateHost {
  currentVersion: string;
  isPackaged: boolean;
  platform: NodeJS.Platform;
  userAgent: string;
  updatesDirectory: string;
  fetchText(url: string, options?: { signal?: AbortSignal; headers?: Record<string, string> }): Promise<string>;
  downloadFile(url: string, destination: string, options?: DownloadOptions): Promise<string>;
  spawnInstaller(installerPath: string): Promise<void>;
  quitApp(): void;
  log(line: string): void;
}

export type AppUpdateListener = (status: AppUpdateStatus) => void;

export function parseReleaseVersion(tag: string): string {
  const match = VERSION_PATTERN.exec(tag.trim());
  if (match === null) throw new Error(`unsupported release tag: ${tag}`);
  return `${match[1]}.${match[2]}.${match[3]}`;
}

export function compareVersions(left: string, right: string): number {
  const a = versionParts(left);
  const b = versionParts(right);
  for (let index = 0; index < 3; index += 1) {
    if (a[index] < b[index]) return -1;
    if (a[index] > b[index]) return 1;
  }
  return 0;
}

export function installerAssetName(version: string): string {
  return `ClipHub.Studio.Setup.${parseReleaseVersion(version)}.exe`;
}

// Assisted silent NSIS skips the finish-page Run checkbox; `--force-run` starts the app after replace.
export const INSTALLER_SPAWN_ARGS = ['/S', '--updated', '--force-run'] as const;

export function releaseDownloadUrl(version: string, fileName: string): string {
  const normalized = parseReleaseVersion(version);
  if (fileName !== installerAssetName(normalized) && fileName !== 'SHA256SUMS.txt') {
    throw new Error(`unexpected release asset: ${fileName}`);
  }
  return `https://github.com/rechedev9/cliphub/releases/download/v${normalized}/${fileName}`;
}

export function parseGithubLatestRelease(body: string): string {
  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    throw new Error('GitHub latest release is not JSON');
  }
  if (!isRecord(parsed) || typeof parsed.tag_name !== 'string') {
    throw new Error('GitHub latest release is missing tag_name');
  }
  if (parsed.draft === true || parsed.prerelease === true) {
    throw new Error('GitHub latest release is not a stable build');
  }
  return parseReleaseVersion(parsed.tag_name);
}

export function checksumForFile(text: string, fileName: string): string {
  const lines = text.replace(/^\uFEFF/, '').trimEnd().split(/\r?\n/);
  for (const line of lines) {
    const match = CHECKSUM_LINE.exec(line);
    if (match === null) throw new Error('invalid SHA256SUMS.txt line');
    if (match[2] === fileName) return match[1];
  }
  throw new Error(`SHA256SUMS.txt is missing ${fileName}`);
}

export function digestMatches(got: string, want: string): boolean {
  if (!/^[a-f0-9]{64}$/.test(got) || !/^[a-f0-9]{64}$/.test(want)) return false;
  return timingSafeEqual(Buffer.from(got, 'hex'), Buffer.from(want, 'hex'));
}

export class AppUpdateController {
  private readonly host: AppUpdateHost;
  private statusValue: AppUpdateStatus;
  private readonly listeners = new Set<AppUpdateListener>();
  private readonly work = new AbortController();
  private inFlight: Promise<void> | null = null;
  private lastProgressAt = 0;
  private readyInstaller: { version: string; path: string } | null = null;

  constructor(host: AppUpdateHost) {
    this.host = host;
    this.statusValue = host.isPackaged && host.platform === 'win32'
      ? { state: APP_UPDATE_STATE.idle }
      : { state: APP_UPDATE_STATE.unavailable };
  }

  status(): AppUpdateStatus {
    return this.statusValue;
  }

  subscribe(listener: AppUpdateListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  dispose(): void {
    this.work.abort();
    this.listeners.clear();
  }

  async check({ quiet = false }: { quiet?: boolean } = {}): Promise<void> {
    if (this.statusValue.state === APP_UPDATE_STATE.unavailable) return;
    if (this.isBusy()) return;
    await this.run(() => this.performCheck(quiet), quiet);
  }

  async install(): Promise<void> {
    if (this.statusValue.state === APP_UPDATE_STATE.unavailable) return;
    if (this.statusValue.state === APP_UPDATE_STATE.ready) {
      if (this.isBusy()) return;
      await this.run(() => this.performApply());
      return;
    }
    if (this.statusValue.state === APP_UPDATE_STATE.available) {
      if (this.isBusy()) return;
      const version = this.statusValue.version;
      await this.run(() => this.performDownload(version));
    }
  }

  private isBusy(): boolean {
    return this.inFlight !== null
      || this.statusValue.state === APP_UPDATE_STATE.downloading
      || this.statusValue.state === APP_UPDATE_STATE.installing;
  }

  private async run(work: () => Promise<void>, quiet = false): Promise<void> {
    const pending = work().catch((error: unknown) => {
      this.host.log(`[update] ${String(error)}\n`);
      if (quiet) return;
      this.readyInstaller = null;
      this.setStatus({
        state: APP_UPDATE_STATE.error,
        message: userFacingUpdateError(error),
      });
    }).finally(() => {
      if (this.inFlight === pending) this.inFlight = null;
    });
    this.inFlight = pending;
    await pending;
  }

  private async performCheck(quiet: boolean): Promise<void> {
    if (!quiet) this.setStatus({ state: APP_UPDATE_STATE.checking });
    const latest = parseGithubLatestRelease(
      await this.host.fetchText(GITHUB_LATEST_RELEASE_URL, {
        signal: this.work.signal,
        headers: this.apiHeaders(),
      }),
    );
    if (compareVersions(latest, this.host.currentVersion) <= 0) {
      this.readyInstaller = null;
      this.setStatus({ state: APP_UPDATE_STATE.current, version: this.host.currentVersion });
      return;
    }
    if (this.readyInstaller?.version === latest) {
      this.setStatus({ state: APP_UPDATE_STATE.ready, version: latest });
      return;
    }
    this.readyInstaller = null;
    this.setStatus({
      state: APP_UPDATE_STATE.available,
      version: latest,
      currentVersion: this.host.currentVersion,
    });
  }

  private async performDownload(version: string): Promise<void> {
    const fileName = installerAssetName(version);
    const installerUrl = releaseDownloadUrl(version, fileName);
    const checksumUrl = releaseDownloadUrl(version, 'SHA256SUMS.txt');
    this.setStatus({
      state: APP_UPDATE_STATE.downloading,
      version,
      received: 0,
      total: null,
    });

    const sums = await this.host.fetchText(checksumUrl, {
      signal: this.work.signal,
      headers: this.downloadHeaders(),
    });
    const expected = checksumForFile(sums, fileName);

    fs.rmSync(this.host.updatesDirectory, { recursive: true, force: true });
    fs.mkdirSync(this.host.updatesDirectory, { recursive: true });
    const destination = path.join(this.host.updatesDirectory, fileName);
    const digest = await this.host.downloadFile(installerUrl, destination, {
      signal: this.work.signal,
      headers: this.downloadHeaders(),
      onProgress: (received, total) => this.reportProgress(version, received, total),
    });
    if (!digestMatches(digest, expected)) {
      fs.rmSync(destination, { force: true });
      throw new Error('installer sha256 mismatch');
    }
    this.readyInstaller = { version, path: destination };
    this.setStatus({ state: APP_UPDATE_STATE.ready, version });
  }

  private async performApply(): Promise<void> {
    const ready = this.readyInstaller;
    if (ready === null || this.statusValue.state !== APP_UPDATE_STATE.ready) {
      throw new Error('no verified installer is ready');
    }
    if (path.basename(ready.path) !== installerAssetName(ready.version)) {
      throw new Error('installer path does not match the verified asset');
    }
    this.setStatus({ state: APP_UPDATE_STATE.installing, version: ready.version });
    await this.host.spawnInstaller(ready.path);
    this.host.quitApp();
  }

  private reportProgress(version: string, received: number, total: number | undefined): void {
    const now = Date.now();
    if (now - this.lastProgressAt < PROGRESS_THROTTLE_MS && total !== undefined && received < total) {
      return;
    }
    this.lastProgressAt = now;
    this.setStatus({
      state: APP_UPDATE_STATE.downloading,
      version,
      received,
      total: total ?? null,
    });
  }

  private apiHeaders(): Record<string, string> {
    return {
      Accept: 'application/vnd.github+json',
      'User-Agent': this.host.userAgent,
    };
  }

  private downloadHeaders(): Record<string, string> {
    return { 'User-Agent': this.host.userAgent };
  }

  private setStatus(status: AppUpdateStatus): void {
    this.statusValue = status;
    for (const listener of this.listeners) listener(status);
  }
}

export function createDefaultAppUpdateHost(input: {
  currentVersion: string;
  isPackaged: boolean;
  platform: NodeJS.Platform;
  updatesDirectory: string;
  spawnInstaller: (installerPath: string) => Promise<void>;
  quitApp: () => void;
  log: (line: string) => void;
}): AppUpdateHost {
  const userAgent = `ClipHub-Studio/${input.currentVersion}`;
  return {
    ...input,
    userAgent,
    fetchText: (url, options) => fetchText(url, options),
    downloadFile,
  };
}

function versionParts(version: string): [number, number, number] {
  const match = VERSION_PATTERN.exec(version);
  if (match === null) throw new Error(`unsupported version: ${version}`);
  return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function userFacingUpdateError(error: unknown): string {
  const text = error instanceof Error ? error.message : String(error);
  if (text.includes('sha256 mismatch')) {
    return 'El instalador no coincide con SHA256SUMS.txt. Vuelve a intentarlo.';
  }
  if (text.includes('HTTP 403') || text.includes('HTTP 429')) {
    return 'GitHub ha limitado la consulta. Prueba de nuevo en unos minutos.';
  }
  if (text.includes('HTTP ') || text.includes('timed out') || text.includes('ENOTFOUND')) {
    return 'No se ha podido consultar GitHub Releases. Comprueba la red y reintenta.';
  }
  return 'No se ha podido actualizar ClipHub Studio. Reintenta o descarga el instalador desde GitHub Releases.';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
