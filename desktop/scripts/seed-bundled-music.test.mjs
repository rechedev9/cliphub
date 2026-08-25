import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import {
  extractBundledMusic,
  installerAssetUrl,
  localOnlyTracks,
  missingTrackFiles,
  resolveSevenZip,
  seedBundledMusic,
} from './seed-bundled-music.mjs';

test('classifies catalog tracks that must be bundled in the installer', () => {
  const cases = [
    {
      catalog: { tracks: [{ id: 'reggaeton-1', ext: 'mp3' }] },
      want: ['reggaeton-1'],
    },
    {
      catalog: { tracks: [{ id: 'remote', ext: 'mp3', downloadUrl: 'https://example.test/a.mp3' }] },
      want: [],
    },
    {
      catalog: { tracks: [{ id: '', ext: 'mp3' }, { ext: 'mp3' }, null] },
      want: [],
    },
  ];

  for (const { catalog, want } of cases) {
    assert.deepEqual(localOnlyTracks(catalog).map((track) => track.id), want);
  }
});

test('lists only the local-only audio files that are absent', () => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-music-missing-'));
  try {
    writeFileSync(join(directory, 'have.mp3'), 'ok');
    const missing = missingTrackFiles([
      { id: 'have', ext: 'mp3' },
      { id: 'need', ext: 'mp3' },
    ], directory);
    assert.deepEqual(missing.map((track) => track.id), ['need']);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('picks the Setup exe from the latest GitHub release', () => {
  const url = installerAssetUrl({
    assets: [
      { name: 'SHA256SUMS.txt', browser_download_url: 'https://example.test/SHA256SUMS.txt' },
      {
        name: 'ClipHub.Studio.Setup.2.4.31.exe',
        browser_download_url: 'https://example.test/ClipHub.Studio.Setup.2.4.31.exe',
      },
    ],
  });
  assert.deepEqual(url, {
    name: 'ClipHub.Studio.Setup.2.4.31.exe',
    url: 'https://example.test/ClipHub.Studio.Setup.2.4.31.exe',
  });
  assert.throws(() => installerAssetUrl({ assets: [] }), /missing ClipHub.Studio.Setup/);
});

test('resolves 7-Zip from SEVEN_ZIP or the Windows install path', () => {
  assert.equal(
    resolveSevenZip({ SEVEN_ZIP: 'C:\\tools\\7z.exe' }, (path) => path === 'C:\\tools\\7z.exe'),
    'C:\\tools\\7z.exe',
  );
  assert.equal(
    resolveSevenZip({}, (path) => path === join('C:\\', 'Program Files', '7-Zip', '7z.exe')),
    join('C:\\', 'Program Files', '7-Zip', '7z.exe'),
  );
});

test('extracts only the inner app-64.7z music payload', () => {
  const calls = [];
  const destination = mkdtempSync(join(tmpdir(), 'cliphub-music-out-'));
  try {
    extractBundledMusic('installer.exe', destination, (_command, args) => {
      calls.push(args);
      if (args.includes('$PLUGINSDIR/app-64.7z')) {
        const out = args.find((arg) => arg.startsWith('-o')).slice(2);
        writeFileSync(join(out, 'app-64.7z'), 'packed');
      }
    }, '7z');
    assert.equal(calls[0].includes('$PLUGINSDIR/app-64.7z'), true);
    assert.equal(calls[1].includes('resources/music/*.mp3'), true);
    assert.equal(calls[1].includes(`-o${destination}`), true);
  } finally {
    rmSync(destination, { recursive: true, force: true });
  }
});

test('seeds missing local-only tracks from the latest published installer', async () => {
  const musicDirectory = mkdtempSync(join(tmpdir(), 'cliphub-music-seed-'));
  writeFileSync(join(musicDirectory, 'catalog.json'), JSON.stringify({
    tracks: [{ id: 'reggaeton-1', ext: 'mp3' }],
  }));
  const fetches = [];
  const result = await seedBundledMusic({
    musicDirectory,
    catalogPath: join(musicDirectory, 'catalog.json'),
    async fetchImpl(url) {
      fetches.push(url);
      if (url.includes('releases/latest')) {
        return {
          ok: true,
          json: async () => ({
            assets: [{
              name: 'ClipHub.Studio.Setup.2.4.31.exe',
              browser_download_url: 'https://example.test/ClipHub.Studio.Setup.2.4.31.exe',
            }],
          }),
        };
      }
      return {
        ok: true,
        arrayBuffer: async () => Buffer.from('installer'),
      };
    },
    extract(_installerPath, destination) {
      writeFileSync(join(destination, 'reggaeton-1.mp3'), 'audio');
    },
  });
  assert.deepEqual(result, { seeded: true, missing: ['reggaeton-1'] });
  assert.equal(readFileSync(join(musicDirectory, 'reggaeton-1.mp3'), 'utf8'), 'audio');
  assert.equal(fetches[0], 'https://api.github.com/repos/rechedev9/cliphub/releases/latest');
  rmSync(musicDirectory, { recursive: true, force: true });
});

test('does not download an installer when every bundled track is already present', async () => {
  const musicDirectory = mkdtempSync(join(tmpdir(), 'cliphub-music-present-'));
  writeFileSync(join(musicDirectory, 'catalog.json'), JSON.stringify({
    tracks: [{ id: 'reggaeton-1', ext: 'mp3' }],
  }));
  writeFileSync(join(musicDirectory, 'reggaeton-1.mp3'), 'audio');
  let fetched = false;
  const result = await seedBundledMusic({
    musicDirectory,
    catalogPath: join(musicDirectory, 'catalog.json'),
    async fetchImpl() {
      fetched = true;
      throw new Error('should not fetch');
    },
  });
  assert.deepEqual(result, { seeded: false, missing: [] });
  assert.equal(fetched, false);
  rmSync(musicDirectory, { recursive: true, force: true });
});
