import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { MUSIC_RELEASE_URL, localOnlyTracks, seedBundledMusic, tracksToFetch } from './seed-bundled-music.mjs';

const sha = (text) => createHash('sha256').update(text).digest('hex');

function musicDir(tracks) {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-music-'));
  writeFileSync(join(directory, 'catalog.json'), JSON.stringify({ tracks }));
  return directory;
}

function serve(files, fetches) {
  return async (url) => {
    fetches.push(url);
    const name = url.slice(url.lastIndexOf('/') + 1);
    if (!(name in files)) return { ok: false, status: 404 };
    return { ok: true, arrayBuffer: async () => Buffer.from(files[name]) };
  };
}

test('classifies catalog tracks that must be bundled in the installer', () => {
  const cases = [
    { catalog: { tracks: [{ id: 'reggaeton-1', ext: 'mp3' }] }, want: ['reggaeton-1'] },
    { catalog: { tracks: [{ id: 'remote', ext: 'mp3', downloadUrl: 'https://example.test/a.mp3' }] }, want: [] },
    { catalog: { tracks: [{ id: '', ext: 'mp3' }, { ext: 'mp3' }, null] }, want: [] },
  ];
  for (const { catalog, want } of cases) {
    assert.deepEqual(localOnlyTracks(catalog).map((track) => track.id), want);
  }
});

test('fetches the tracks that are absent or differ from the catalog digest', () => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-music-missing-'));
  try {
    writeFileSync(join(directory, 'good.mp3'), 'ok');
    writeFileSync(join(directory, 'stale.mp3'), 'old');
    const fetch = tracksToFetch([
      { id: 'good', ext: 'mp3', sha256: sha('ok') },
      { id: 'stale', ext: 'mp3', sha256: sha('new') },
      { id: 'need', ext: 'mp3', sha256: sha('x') },
    ], directory);
    assert.deepEqual(fetch.map((track) => track.id), ['stale', 'need']);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('downloads a missing track from the pinned music release', async () => {
  const directory = musicDir([{ id: 'reggaeton-1', ext: 'mp3', sha256: sha('audio') }]);
  try {
    const fetches = [];
    const result = await seedBundledMusic({ musicDirectory: directory, fetchImpl: serve({ 'reggaeton-1.mp3': 'audio' }, fetches) });
    assert.deepEqual(result, { fetched: ['reggaeton-1'] });
    assert.deepEqual(fetches, [`${MUSIC_RELEASE_URL}/reggaeton-1.mp3`]);
    assert.equal(readFileSync(join(directory, 'reggaeton-1.mp3'), 'utf8'), 'audio');
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('refuses a download whose digest does not match and keeps no file', async () => {
  const directory = musicDir([{ id: 'reggaeton-1', ext: 'mp3', sha256: sha('audio') }]);
  try {
    await assert.rejects(
      seedBundledMusic({ musicDirectory: directory, fetchImpl: serve({ 'reggaeton-1.mp3': 'tampered' }, []) }),
      /catalog\.json pins/,
    );
    assert.throws(() => readFileSync(join(directory, 'reggaeton-1.mp3')));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('fails before any download when a local-only track has no sha256', async () => {
  const directory = musicDir([{ id: 'reggaeton-1', ext: 'mp3' }]);
  try {
    const fetches = [];
    await assert.rejects(
      seedBundledMusic({ musicDirectory: directory, fetchImpl: serve({}, fetches) }),
      /has no valid sha256/,
    );
    assert.deepEqual(fetches, []);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('does not fetch when every bundled track already matches', async () => {
  const directory = musicDir([{ id: 'reggaeton-1', ext: 'mp3', sha256: sha('audio') }]);
  try {
    writeFileSync(join(directory, 'reggaeton-1.mp3'), 'audio');
    const fetches = [];
    const result = await seedBundledMusic({ musicDirectory: directory, fetchImpl: serve({}, fetches) });
    assert.deepEqual(result, { fetched: [] });
    assert.deepEqual(fetches, []);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('every local-only track in the real catalog pins a sha256', () => {
  const catalog = JSON.parse(readFileSync(new URL('../../data/music/catalog.json', import.meta.url), 'utf8'));
  for (const track of localOnlyTracks(catalog)) {
    assert.match(track.sha256 ?? '', /^[0-9a-f]{64}$/, `${track.id} needs a sha256`);
  }
});
