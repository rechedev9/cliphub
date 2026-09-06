export type PlaybackVideoDecode = 'hardware-accelerated' | 'software-only' | 'unavailable' | 'unknown';

export type PlaybackDiagnostics =
  | {
    available: false;
    state: 'initializing';
  }
  | {
    available: true;
    state: 'ready';
    electronVersion: string;
    chromiumVersion: string;
    hardwareAcceleration: 'enabled' | 'disabled';
    videoDecode: PlaybackVideoDecode;
    scope: 'global';
  };

interface PlaybackDiagnosticsInput {
  gpuInformationReady: boolean;
  electronVersion: unknown;
  chromiumVersion: unknown;
  hardwareAccelerationEnabled: boolean;
  videoDecodeStatus: unknown;
}

const HARDWARE_VIDEO_DECODE = new Set([
  'enabled',
  'enabled_force',
  'enabled_force_on',
  'enabled_on',
  'enabled_readback',
]);
const SOFTWARE_VIDEO_DECODE = new Set(['disabled_software', 'unavailable_software']);
const UNAVAILABLE_VIDEO_DECODE = new Set([
  'disabled_off',
  'disabled_off_ok',
  'unavailable_off',
  'unavailable_off_ok',
]);

export const PLAYBACK_INFO_TTL_MS = 30_000;

export type GPUFeatureProbe = () => {
  hardwareAccelerationEnabled: boolean;
  videoDecodeStatus: unknown;
};

export type PlaybackInfoCache = {
  at: number;
  ready: boolean;
  info: PlaybackDiagnostics;
};

/** Reuses a ready GPU summary inside the TTL so Settings remounts skip a rescan. */
export function readPlaybackInfo(
  ready: boolean,
  versions: { electron: unknown; chromium: unknown },
  probe: GPUFeatureProbe,
  cache: PlaybackInfoCache | null,
  now: number,
  ttlMs = PLAYBACK_INFO_TTL_MS,
): { info: PlaybackDiagnostics; cache: PlaybackInfoCache } {
  if (cache !== null && cache.ready === ready && cache.info.available && now - cache.at < ttlMs) {
    return { info: cache.info, cache };
  }
  const probed = ready ? probe() : { hardwareAccelerationEnabled: false, videoDecodeStatus: null };
  const info = createPlaybackDiagnostics({
    gpuInformationReady: ready,
    electronVersion: versions.electron,
    chromiumVersion: versions.chromium,
    hardwareAccelerationEnabled: probed.hardwareAccelerationEnabled,
    videoDecodeStatus: probed.videoDecodeStatus,
  });
  return { info, cache: { at: now, ready, info } };
}

/** Builds the only GPU-derived summary allowed to cross the settings bridge. */
export function createPlaybackDiagnostics(input: PlaybackDiagnosticsInput): PlaybackDiagnostics {
  if (!input.gpuInformationReady) return { available: false, state: 'initializing' };

  return {
    available: true,
    state: 'ready',
    electronVersion: boundedVersion(input.electronVersion),
    chromiumVersion: boundedVersion(input.chromiumVersion),
    hardwareAcceleration: input.hardwareAccelerationEnabled ? 'enabled' : 'disabled',
    videoDecode: normalizeVideoDecode(input.videoDecodeStatus),
    scope: 'global',
  };
}

function normalizeVideoDecode(value: unknown): PlaybackVideoDecode {
  if (typeof value !== 'string') return 'unknown';
  if (HARDWARE_VIDEO_DECODE.has(value)) return 'hardware-accelerated';
  if (SOFTWARE_VIDEO_DECODE.has(value)) return 'software-only';
  if (UNAVAILABLE_VIDEO_DECODE.has(value)) return 'unavailable';
  return 'unknown';
}

function boundedVersion(value: unknown): string {
  if (typeof value !== 'string' || value.length === 0 || value.length > 64) return 'unknown';
  return /^[0-9A-Za-z.+_-]+$/.test(value) ? value : 'unknown';
}
