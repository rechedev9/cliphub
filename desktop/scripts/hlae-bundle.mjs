import { createHash } from 'node:crypto';
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const defaultDesktopDirectory = join(here, '..');

export function readPinnedHLAETool(desktopDirectory = defaultDesktopDirectory) {
  const manifestPath = join(desktopDirectory, 'src', 'hlae-tool.json');
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  for (const field of ['version', 'archiveName', 'url', 'sha256', 'treeSha256', 'exeRel']) {
    if (typeof manifest[field] !== 'string' || manifest[field] === '') {
      throw new Error(`[hlae-bundle] invalid ${field} in ${manifestPath}`);
    }
  }
  if (!/^[a-f0-9]{64}$/.test(manifest.sha256)) {
    throw new Error(`[hlae-bundle] invalid sha256 in ${manifestPath}`);
  }
  if (!/^[a-f0-9]{64}$/.test(manifest.treeSha256)) {
    throw new Error(`[hlae-bundle] invalid treeSha256 in ${manifestPath}`);
  }
  if (!/^hlae_[a-zA-Z0-9_]+\.zip$/.test(manifest.archiveName)) {
    throw new Error(`[hlae-bundle] unsafe archiveName in ${manifestPath}`);
  }
  return manifest;
}

// build-resources/ is wiped on every assemble, so a verified copy of the pinned
// archive is kept here and reused before downloading. A cached file that no
// longer matches the pin is ignored, so the digest check stays the only trust
// decision.
export function defaultHLAECacheDirectory(desktopDirectory = defaultDesktopDirectory) {
  return join(desktopDirectory, '.hlae-cache');
}

export async function stageBundledHLAE({
  desktopDirectory = defaultDesktopDirectory,
  destinationDirectory,
  cacheDirectory = defaultHLAECacheDirectory(desktopDirectory),
  fetchImpl = fetch,
  spec = readPinnedHLAETool(desktopDirectory),
}) {
  if (!destinationDirectory) throw new Error('[hlae-bundle] destinationDirectory is required');
  mkdirSync(destinationDirectory, { recursive: true });
  const destination = join(destinationDirectory, spec.archiveName);
  const temporary = `${destination}.tmp`;
  const cached = cacheDirectory ? join(cacheDirectory, spec.archiveName) : '';
  rmSync(temporary, { force: true });

  try {
    const fromCache = cached !== '' && copyVerifiedCache(cached, temporary, spec);
    if (!fromCache) {
      const response = await fetchImpl(spec.url, {
        headers: { 'User-Agent': 'ClipHub-Studio-build' },
        redirect: 'follow',
      });
      if (!response.ok) {
        throw new Error(`[hlae-bundle] download failed with HTTP ${response.status}`);
      }
      writeFileSync(temporary, Buffer.from(await response.arrayBuffer()));
    }
    verifyBundledHLAE(temporary, spec);
    if (cached && !fromCache) writeCache(cacheDirectory, cached, temporary);
    rmSync(destination, { force: true });
    renameSync(temporary, destination);
    return destination;
  } finally {
    rmSync(temporary, { force: true });
  }
}

// The cache is an optimization only: an unreadable or mismatching entry falls
// back to the download, and a failed cache write never fails the staging.
function copyVerifiedCache(cached, temporary, spec) {
  try {
    if (!existsSync(cached) || sha256File(cached) !== spec.sha256) return false;
    copyFileSync(cached, temporary);
    return true;
  } catch (err) {
    console.warn(`[hlae-bundle] ignoring unreadable cache ${cached}: ${String(err)}`);
    rmSync(temporary, { force: true });
    return false;
  }
}

function writeCache(cacheDirectory, cached, source) {
  const partial = `${cached}.tmp`;
  try {
    mkdirSync(cacheDirectory, { recursive: true });
    copyFileSync(source, partial);
    renameSync(partial, cached);
  } catch (err) {
    console.warn(`[hlae-bundle] could not update cache ${cached}: ${String(err)}`);
    rmSync(partial, { force: true });
  }
}

function sha256File(filePath) {
  return createHash('sha256').update(readFileSync(filePath)).digest('hex');
}

export function verifyBundledHLAE(archivePath, spec = readPinnedHLAETool()) {
  if (!existsSync(archivePath)) {
    throw new Error(`[hlae-bundle] missing bundled archive ${archivePath}`);
  }
  const digest = sha256File(archivePath);
  if (digest !== spec.sha256) {
    throw new Error(`[hlae-bundle] sha256 mismatch: got ${digest}, want ${spec.sha256}`);
  }
  return archivePath;
}
