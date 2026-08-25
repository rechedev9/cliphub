import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, rmSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const desktop = join(here, '..');
const repo = join(desktop, '..');
const LATEST_RELEASE_URL = 'https://api.github.com/repos/rechedev9/cliphub/releases/latest';
const INSTALLER_NAME = /^ClipHub\.Studio\.Setup\.\d+\.\d+\.\d+\.exe$/;

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

export function missingTrackFiles(tracks, musicDirectory) {
  return tracks.filter((track) => !existsSync(join(musicDirectory, `${track.id}.${track.ext}`)));
}

export function installerAssetUrl(release) {
  if (!release || !Array.isArray(release.assets)) {
    throw new Error('GitHub latest release is missing assets');
  }
  const asset = release.assets.find((entry) => typeof entry?.name === 'string' && INSTALLER_NAME.test(entry.name));
  if (typeof asset?.browser_download_url !== 'string' || asset.browser_download_url === '') {
    throw new Error('GitHub latest release is missing ClipHub.Studio.Setup.<version>.exe');
  }
  return { name: asset.name, url: asset.browser_download_url };
}

export function resolveSevenZip(environment = process.env, pathExists = existsSync) {
  const configured = environment.SEVEN_ZIP;
  if (typeof configured === 'string' && configured !== '') return configured;
  for (const candidate of [
    join('C:\\', 'Program Files', '7-Zip', '7z.exe'),
    join('C:\\', 'Program Files (x86)', '7-Zip', '7z.exe'),
  ]) {
    if (pathExists(candidate)) return candidate;
  }
  return '7z';
}

export function extractBundledMusic(installerPath, destinationDirectory, runFile = execFileSync, sevenZip = resolveSevenZip()) {
  const staging = mkdtempSync(join(tmpdir(), 'cliphub-music-seed-'));
  try {
    runFile(sevenZip, ['e', '-y', `-o${staging}`, installerPath, '$PLUGINSDIR/app-64.7z'], { stdio: 'pipe' });
    const packedApp = join(staging, 'app-64.7z');
    if (!existsSync(packedApp)) throw new Error('published installer is missing $PLUGINSDIR/app-64.7z');
    mkdirSync(destinationDirectory, { recursive: true });
    runFile(sevenZip, ['e', '-y', `-o${destinationDirectory}`, packedApp, 'resources/music/*.mp3'], { stdio: 'pipe' });
  } finally {
    rmSync(staging, { recursive: true, force: true });
  }
}

export async function seedBundledMusic({
  musicDirectory = join(repo, 'data', 'music'),
  catalogPath = join(musicDirectory, 'catalog.json'),
  fetchImpl = fetch,
  extract = extractBundledMusic,
} = {}) {
  const catalog = JSON.parse(readFileSync(catalogPath, 'utf8'));
  const missing = missingTrackFiles(localOnlyTracks(catalog), musicDirectory);
  if (missing.length === 0) return { seeded: false, missing: [] };

  const release = await fetchGithubJson(LATEST_RELEASE_URL, fetchImpl);
  const installer = installerAssetUrl(release);
  const downloadDir = mkdtempSync(join(tmpdir(), 'cliphub-installer-seed-'));
  try {
    const installerPath = join(downloadDir, installer.name);
    await downloadFile(installer.url, installerPath, fetchImpl);
    extract(installerPath, musicDirectory);
  } finally {
    rmSync(downloadDir, { recursive: true, force: true });
  }

  const stillMissing = missingTrackFiles(missing, musicDirectory);
  if (stillMissing.length > 0) {
    throw new Error(`seeded installer is missing ${stillMissing.map((track) => track.id).join(', ')}`);
  }
  return { seeded: true, missing: missing.map((track) => track.id) };
}

async function fetchGithubJson(url, fetchImpl) {
  const response = await fetchImpl(url, {
    headers: {
      Accept: 'application/vnd.github+json',
      'User-Agent': 'ClipHub-Studio-build',
    },
  });
  if (!response.ok) throw new Error(`GET ${url}: HTTP ${response.status}`);
  return response.json();
}

async function downloadFile(url, destination, fetchImpl) {
  const response = await fetchImpl(url, {
    headers: { 'User-Agent': 'ClipHub-Studio-build' },
    redirect: 'follow',
  });
  if (!response.ok) throw new Error(`GET ${url}: HTTP ${response.status}`);
  writeFileSync(destination, Buffer.from(await response.arrayBuffer()));
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const result = await seedBundledMusic();
  if (result.seeded) {
    console.log(`[music] seeded ${result.missing.length} local-only tracks from the last published installer`);
  } else {
    console.log('[music] bundled local-only tracks already present');
  }
  if (existsSync(join(repo, 'data', 'music'))) {
    const names = readdirSync(join(repo, 'data', 'music')).filter((name) => name.endsWith('.mp3'));
    console.log(`[music] ${names.length} mp3 files in data/music`);
  }
}
