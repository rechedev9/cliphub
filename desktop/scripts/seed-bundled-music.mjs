// Fetches the gitignored local-only Suno tracks that the installer bundles from the pinned
// music-assets release, checking each file against the sha256 in data/music/catalog.json.
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, readdirSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repo = join(here, '..', '..');
const SHA256_RE = /^[0-9a-f]{64}$/;
// A new or changed track goes to a new music-assets-N release, and this pin moves with it.
export const MUSIC_RELEASE_URL = 'https://github.com/rechedev9/cliphub/releases/download/music-assets-1';

export function localOnlyTracks(catalog) {
  return (catalog.tracks ?? []).filter((track) => (
    track
    && typeof track.id === 'string'
    && track.id !== ''
    && typeof track.ext === 'string'
    && track.ext !== ''
    && !track.downloadUrl
  ));
}

export function fileSha256(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

export function trackDigest(track) {
  if (typeof track.sha256 !== 'string' || !SHA256_RE.test(track.sha256)) {
    throw new Error(`local-only track ${track.id} has no valid sha256 in catalog.json`);
  }
  return track.sha256;
}

export function tracksToFetch(tracks, musicDirectory) {
  return tracks.filter((track) => {
    const path = join(musicDirectory, `${track.id}.${track.ext}`);
    return !existsSync(path) || fileSha256(path) !== trackDigest(track);
  });
}

export async function seedBundledMusic({
  musicDirectory = join(repo, 'data', 'music'),
  catalogPath = join(musicDirectory, 'catalog.json'),
  fetchImpl = fetch,
} = {}) {
  const catalog = JSON.parse(readFileSync(catalogPath, 'utf8'));
  const tracks = localOnlyTracks(catalog);
  tracks.forEach(trackDigest);
  const fetched = tracksToFetch(tracks, musicDirectory);
  mkdirSync(musicDirectory, { recursive: true });
  for (const track of fetched) {
    const name = `${track.id}.${track.ext}`;
    const body = await downloadBytes(`${MUSIC_RELEASE_URL}/${name}`, fetchImpl);
    const digest = createHash('sha256').update(body).digest('hex');
    if (digest !== track.sha256) {
      throw new Error(`${name} from ${MUSIC_RELEASE_URL} has sha256 ${digest}, catalog.json pins ${track.sha256}`);
    }
    const partial = join(musicDirectory, `${name}.partial`);
    writeFileSync(partial, body);
    rmSync(join(musicDirectory, name), { force: true });
    renameSync(partial, join(musicDirectory, name));
  }
  return { fetched: fetched.map((track) => track.id) };
}

async function downloadBytes(url, fetchImpl) {
  const response = await fetchImpl(url, {
    headers: { 'User-Agent': 'ClipHub-Studio-build' },
    redirect: 'follow',
  });
  if (!response.ok) throw new Error(`GET ${url}: HTTP ${response.status}`);
  return Buffer.from(await response.arrayBuffer());
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const result = await seedBundledMusic();
  console.log(`[music] fetched ${result.fetched.length} local-only tracks from ${MUSIC_RELEASE_URL}`);
  const names = readdirSync(join(repo, 'data', 'music')).filter((name) => name.endsWith('.mp3'));
  console.log(`[music] ${names.length} mp3 files in data/music`);
}
