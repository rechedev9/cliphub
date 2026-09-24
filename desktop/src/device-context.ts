import { execFile } from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';

// One bucketed hardware line per session for the device.context log record.
// Every value is an enum, a bucket or a v-prefixed version: no user names,
// host names or paths, and nothing the diagnostic filter would rewrite.

const EDITION_TIMEOUT_MS = 3_000;
const GPU_TIMEOUT_MS = 3_000;
const ENCODERS_TIMEOUT_MS = 15_000;
const CPU_MODEL_MAX = 48;
// diagnostic-message.ts redacts any run of 40+ [a-z0-9_-] as an identifier.
const IDENTIFIER_RUN = /[A-Za-z0-9_-]{40,}/;
const REGISTRY_KEY = 'HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion';
const UNKNOWN = 'unknown';

export interface DeviceStatfs {
  bavail: number | bigint;
  bsize: number | bigint;
}

export interface DeviceContextOptions {
  /** Studio data directory; the free-space bucket describes its volume. */
  dataDir: string;
  /** Runtime tools root; pinned versions are read from its <tool>/<version>/ layout. */
  toolsDir: string;
  /** Resolved executables from provisioning; absent when the tool is unconfigured. */
  ffmpegPath?: string;
  hlaePath?: string;
  /** Holds the FFmpeg encoder probe so it runs once per install version. */
  cachePath: string;
  appVersion: string;
  /** process.getSystemVersion() in Electron. */
  systemVersion: () => string;
  /** app.getGPUInfo('basic') in Electron. */
  gpuInfo: () => Promise<unknown>;
  platform?: NodeJS.Platform;
  cpus?: () => ReadonlyArray<{ model: string }>;
  totalMemoryBytes?: () => number;
  statfs?: (target: string) => Promise<DeviceStatfs>;
  runCommand?: (file: string, args: readonly string[], timeoutMs: number) => Promise<string>;
}

export interface DeviceContextFields {
  win_build: string;
  edition: string;
  gpu_vendor: string;
  gpu_device: string;
  gpu_driver: string;
  cpu_cores: string;
  cpu_model: string;
  ram_gb: string;
  disk_free_gb: string;
  ffmpeg: string;
  aac_mf: string;
  hlae: string;
}

const FIELD_ORDER: ReadonlyArray<keyof DeviceContextFields> = [
  'win_build', 'edition', 'gpu_vendor', 'gpu_device', 'gpu_driver', 'cpu_cores',
  'cpu_model', 'ram_gb', 'disk_free_gb', 'ffmpeg', 'aac_mf', 'hlae',
];

interface EncoderCache {
  appVersion: string;
  ffmpeg: string;
  aacMF: 'yes' | 'no';
}

/** Collects the device.context line; every probe is bounded and falls back to "unknown". */
export async function collectDeviceContext(options: DeviceContextOptions): Promise<string> {
  const platform = options.platform ?? process.platform;
  const run = options.runCommand ?? runCommand;
  const ffmpegVersion = options.ffmpegPath ? toolVersion(options.toolsDir, 'ffmpeg', options.ffmpegPath) : 'none';
  const hlaeVersion = options.hlaePath ? toolVersion(options.toolsDir, 'hlae', options.hlaePath) : 'none';
  const [edition, gpu, diskFree, aacMF] = await Promise.all([
    platform === 'win32' ? windowsEdition(run) : Promise.resolve(UNKNOWN),
    gpuFields(options.gpuInfo),
    diskFreeBucket(options.dataDir, options.statfs ?? statfs),
    aacMediaFoundation(options, ffmpegVersion, run),
  ]);
  const cpus = safe(() => (options.cpus ?? os.cpus)(), []);
  return formatDeviceContext({
    win_build: versionValue(safe(options.systemVersion, '')),
    edition,
    ...gpu,
    cpu_cores: cpus.length > 0 ? String(cpus.length) : UNKNOWN,
    cpu_model: normalizeCPUModel(cpus[0]?.model ?? ''),
    ram_gb: ramBucket(safe(options.totalMemoryBytes ?? os.totalmem, 0)),
    disk_free_gb: diskFree,
    ffmpeg: ffmpegVersion,
    aac_mf: aacMF,
    hlae: hlaeVersion,
  });
}

export function formatDeviceContext(fields: DeviceContextFields): string {
  return FIELD_ORDER.map((key) => `${key}=${fields[key] || UNKNOWN}`).join(' ');
}

/** PCI vendor ids reported by Chromium's basic GPU info. */
export function gpuVendor(vendorID: unknown): string {
  if (typeof vendorID !== 'number' || !Number.isInteger(vendorID) || vendorID <= 0) return UNKNOWN;
  if (vendorID === 0x10de) return 'nvidia';
  if (vendorID === 0x1002 || vendorID === 0x1022) return 'amd';
  if (vendorID === 0x8086) return 'intel';
  return 'other';
}

/** Bucket of physical memory; installed RAM reads slightly below its nominal size. */
export function ramBucket(bytes: number): string {
  const gib = bytes / 2 ** 30;
  if (!Number.isFinite(gib) || gib <= 0) return UNKNOWN;
  if (gib >= 57) return '64+';
  if (gib >= 28) return '32';
  if (gib >= 14) return '16';
  if (gib >= 7) return '8';
  return '4';
}

export function diskBucket(freeBytes: number): string {
  const gib = freeBytes / 2 ** 30;
  if (!Number.isFinite(gib) || gib < 0) return UNKNOWN;
  if (gib < 10) return '<10';
  if (gib < 50) return '10-50';
  if (gib < 200) return '50-200';
  return '200+';
}

/** "Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz" -> "Intel_Core_i7-10700K_CPU_3.80GHz". */
export function normalizeCPUModel(model: string): string {
  const words = model
    .replace(/\((?:R|TM|C)\)/gi, ' ')
    .replace(/[^A-Za-z0-9.\- ]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  let value = words.join('_').slice(0, CPU_MODEL_MAX);
  // A long unbroken name would reach the collector as "[identifier]".
  if (IDENTIFIER_RUN.test(value)) value = value.slice(0, 39);
  return value.replace(/[_.-]+$/, '') || UNKNOWN;
}

/**
 * v-prefixed so the IPv4 redaction rule cannot match a 4-part version. A
 * named pin suffix ("2.192.2-cliphub.1") is kept; a git-describe tail
 * ("n8.1.2-30-g45f1910444-20260723") is not.
 */
export function versionValue(raw: string): string {
  const match = /^[vn]?(\d+(?:\.\d+){0,3}(?:-[A-Za-z][0-9A-Za-z]*(?:\.[0-9A-Za-z]+)*)?)/i.exec(raw.trim());
  return match ? `v${match[1]}` : UNKNOWN;
}

/** Version directory of a provisioned tool (<toolsDir>/<name>/<version>/...). */
export function toolVersion(toolsDir: string, name: string, executable: string): string {
  const relative = path.relative(path.join(toolsDir, name), executable);
  const version = relative.split(/[\\/]/)[0] ?? '';
  if (!relative || relative.startsWith('..') || path.isAbsolute(relative) || version === relative) return UNKNOWN;
  return versionValue(version);
}

export function parseEditionID(output: string): string {
  const edition = /EditionID\s+REG_SZ\s+([A-Za-z0-9_-]{1,40})\s*$/im.exec(output)?.[1];
  return edition ?? UNKNOWN;
}

/** aac_mf is listed when the build has the Media Foundation encoder; N/KN editions lack the OS side. */
export function hasAACMediaFoundation(encoders: string): boolean {
  return /^\s*A\S*\s+aac_mf\s/m.test(encoders);
}

async function windowsEdition(run: NonNullable<DeviceContextOptions['runCommand']>): Promise<string> {
  try {
    return parseEditionID(await run('reg', ['query', REGISTRY_KEY, '/v', 'EditionID'], EDITION_TIMEOUT_MS));
  } catch {
    return UNKNOWN;
  }
}

async function gpuFields(gpuInfo: () => Promise<unknown>): Promise<Pick<DeviceContextFields, 'gpu_vendor' | 'gpu_device' | 'gpu_driver'>> {
  const unknown = { gpu_vendor: UNKNOWN, gpu_device: UNKNOWN, gpu_driver: UNKNOWN };
  let info: unknown;
  try {
    info = await withTimeout(gpuInfo(), GPU_TIMEOUT_MS);
  } catch {
    return unknown;
  }
  const devices = isRecord(info) && Array.isArray(info.gpuDevice) ? info.gpuDevice.filter(isRecord) : [];
  const device = devices.find((candidate) => candidate.active === true) ?? devices[0];
  if (device === undefined) return unknown;
  const deviceID = device.deviceId;
  return {
    gpu_vendor: gpuVendor(device.vendorId),
    gpu_device: typeof deviceID === 'number' && Number.isInteger(deviceID) && deviceID >= 0
      ? `0x${deviceID.toString(16).padStart(4, '0')}`
      : UNKNOWN,
    gpu_driver: typeof device.driverVersion === 'string' ? versionValue(device.driverVersion) : UNKNOWN,
  };
}

async function diskFreeBucket(dataDir: string, read: NonNullable<DeviceContextOptions['statfs']>): Promise<string> {
  // The data dir may not exist yet on a first boot; its volume is what matters.
  let target = path.resolve(dataDir);
  for (;;) {
    try {
      const stats = await read(target);
      return diskBucket(Number(stats.bavail) * Number(stats.bsize));
    } catch {
      const parent = path.dirname(target);
      if (parent === target) return UNKNOWN;
      target = parent;
    }
  }
}

async function aacMediaFoundation(
  options: DeviceContextOptions,
  ffmpegVersion: string,
  run: NonNullable<DeviceContextOptions['runCommand']>,
): Promise<string> {
  if (!options.ffmpegPath) return UNKNOWN;
  const cached = readEncoderCache(options.cachePath);
  if (cached !== null && cached.appVersion === options.appVersion && cached.ffmpeg === ffmpegVersion) return cached.aacMF;
  let output: string;
  try {
    output = await run(options.ffmpegPath, ['-hide_banner', '-encoders'], ENCODERS_TIMEOUT_MS);
  } catch {
    return UNKNOWN;
  }
  const aacMF = hasAACMediaFoundation(output) ? 'yes' : 'no';
  try {
    fs.mkdirSync(path.dirname(options.cachePath), { recursive: true });
    const cache: EncoderCache = { appVersion: options.appVersion, ffmpeg: ffmpegVersion, aacMF };
    fs.writeFileSync(options.cachePath, `${JSON.stringify(cache)}\n`, { encoding: 'utf8', mode: 0o600 });
  } catch {
    // The probe reruns next session when its result cannot be cached.
  }
  return aacMF;
}

function readEncoderCache(cachePath: string): EncoderCache | null {
  try {
    const value: unknown = JSON.parse(fs.readFileSync(cachePath, 'utf8'));
    if (isRecord(value) && typeof value.appVersion === 'string' && typeof value.ffmpeg === 'string'
      && (value.aacMF === 'yes' || value.aacMF === 'no')) {
      return { appVersion: value.appVersion, ffmpeg: value.ffmpeg, aacMF: value.aacMF };
    }
  } catch {
    // Missing or corrupt cache: probe again.
  }
  return null;
}

function runCommand(file: string, args: readonly string[], timeoutMs: number): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(file, [...args], { timeout: timeoutMs, windowsHide: true, maxBuffer: 4 * 1024 * 1024, encoding: 'utf8' },
      (error, stdout) => {
        if (error) reject(error);
        else resolve(stdout);
      });
  });
}

function statfs(target: string): Promise<DeviceStatfs> {
  return fs.promises.statfs(target);
}

function withTimeout<T>(promise: Promise<T>, timeoutMs: number): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timed out')), timeoutMs);
    timer.unref?.();
    promise.then(
      (value) => { clearTimeout(timer); resolve(value); },
      (error: unknown) => { clearTimeout(timer); reject(error); },
    );
  });
}

function safe<T>(read: () => T, fallback: T): T {
  try {
    return read();
  } catch {
    return fallback;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
