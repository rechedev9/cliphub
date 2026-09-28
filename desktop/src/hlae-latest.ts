import { fetchText } from './http-download.ts';

// CS2 updates break AfxHookSource2 every few days and advancedfx usually ships
// a fixed build within a day. Studio therefore runs the latest official HLAE
// release instead of waiting for a pin bump plus a Studio release.
export const ADVANCEDFX_LATEST_RELEASE_URL = 'https://api.github.com/repos/advancedfx/advancedfx/releases/latest';
const ADVANCEDFX_DOWNLOAD_PREFIX = 'https://github.com/advancedfx/advancedfx/releases/download/';
const LATEST_LOOKUP_TIMEOUT_MS = 8_000;

export interface HLAERelease {
  version: string;
  archiveName: string;
  url: string;
  sha256: string;
}

const VERSION_PATTERN = /^(\d+)\.(\d+)\.(\d+)$/;

/** Orders plain x.y.z HLAE versions; returns NaN when either is not one. */
export function compareHLAEVersions(left: string, right: string): number {
  const a = VERSION_PATTERN.exec(left);
  const b = VERSION_PATTERN.exec(right);
  if (!a || !b) return Number.NaN;
  for (let index = 1; index <= 3; index += 1) {
    const difference = Number(a[index]) - Number(b[index]);
    if (difference !== 0) return difference;
  }
  return 0;
}

/**
 * Reads a GitHub "latest release" document. Only a published, non-prerelease
 * advancedfx release with the portable zip and its GitHub sha256 qualifies.
 */
export function parseLatestHLAERelease(value: unknown): HLAERelease | null {
  if (!isRecord(value) || value.draft !== false || value.prerelease !== false) return null;
  if (typeof value.tag_name !== 'string' || !Array.isArray(value.assets)) return null;
  const version = value.tag_name.replace(/^v/, '');
  if (!VERSION_PATTERN.test(version)) return null;
  const archiveName = `hlae_${version.replaceAll('.', '_')}.zip`;
  const asset = value.assets.find((entry) => isRecord(entry) && entry.name === archiveName);
  if (!isRecord(asset) || typeof asset.browser_download_url !== 'string' || typeof asset.digest !== 'string') {
    return null;
  }
  const url = asset.browser_download_url;
  const digest = /^sha256:([a-f0-9]{64})$/.exec(asset.digest);
  if (!url.startsWith(ADVANCEDFX_DOWNLOAD_PREFIX) || !digest) return null;
  return { version, archiveName, url, sha256: digest[1] };
}

/** Asks GitHub for the latest official HLAE; null when offline or unusable. */
export async function fetchLatestHLAERelease(signal?: AbortSignal): Promise<HLAERelease | null> {
  const controller = new AbortController();
  const abort = (): void => controller.abort();
  if (signal?.aborted) controller.abort();
  else signal?.addEventListener('abort', abort, { once: true });
  const timeout = setTimeout(abort, LATEST_LOOKUP_TIMEOUT_MS);
  try {
    const body = await fetchText(ADVANCEDFX_LATEST_RELEASE_URL, {
      signal: controller.signal,
      headers: { accept: 'application/vnd.github+json', 'user-agent': 'ClipHub-Studio' },
    });
    return parseLatestHLAERelease(JSON.parse(body));
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener('abort', abort);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
