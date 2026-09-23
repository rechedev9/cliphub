import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { stageBundledHLAE, verifyBundledHLAE } from './hlae-bundle.mjs';

function fixtureSpec(bytes) {
  return {
    version: 'test',
    archiveName: 'hlae_test.zip',
    url: 'https://example.invalid/hlae_test.zip',
    sha256: createHash('sha256').update(bytes).digest('hex'),
    treeSha256: '0'.repeat(64),
    exeRel: 'HLAE.exe',
  };
}

test('stages and verifies the pinned HLAE archive', async (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-hlae-bundle-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const bytes = Buffer.from('official release fixture');
  const spec = fixtureSpec(bytes);

  const archive = await stageBundledHLAE({
    destinationDirectory: directory,
    cacheDirectory: join(directory, 'cache'),
    spec,
    fetchImpl: async () => new Response(bytes, { status: 200 }),
  });

  assert.equal(archive, join(directory, spec.archiveName));
  assert.deepEqual(readFileSync(archive), bytes);
  assert.equal(verifyBundledHLAE(archive, spec), archive);
});

test('rejects a corrupt archive without publishing it', async (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-hlae-bundle-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const spec = fixtureSpec(Buffer.from('expected'));

  await assert.rejects(
    stageBundledHLAE({
      destinationDirectory: directory,
      cacheDirectory: join(directory, 'cache'),
      spec,
      fetchImpl: async () => new Response('corrupt', { status: 200 }),
    }),
    /sha256 mismatch/,
  );

  assert.equal(existsSync(join(directory, spec.archiveName)), false);
  assert.equal(existsSync(join(directory, `${spec.archiveName}.tmp`)), false);
  assert.equal(existsSync(join(directory, 'cache', spec.archiveName)), false);
});

test('reuses a cached archive that matches the pin without downloading', async (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-hlae-bundle-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const bytes = Buffer.from('cached release fixture');
  const spec = fixtureSpec(bytes);
  const cacheDirectory = join(directory, 'cache');
  mkdirSync(cacheDirectory);
  writeFileSync(join(cacheDirectory, spec.archiveName), bytes);

  const archive = await stageBundledHLAE({
    destinationDirectory: join(directory, 'out'),
    cacheDirectory,
    spec,
    fetchImpl: async () => assert.fail('a matching cached archive must not be downloaded'),
  });

  assert.deepEqual(readFileSync(archive), bytes);
});

test('downloads over a cached archive that no longer matches the pin', async (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-hlae-bundle-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const bytes = Buffer.from('pinned release fixture');
  const spec = fixtureSpec(bytes);
  const cacheDirectory = join(directory, 'cache');
  mkdirSync(cacheDirectory);
  writeFileSync(join(cacheDirectory, spec.archiveName), 'previous pin');
  let downloads = 0;

  const archive = await stageBundledHLAE({
    destinationDirectory: join(directory, 'out'),
    cacheDirectory,
    spec,
    fetchImpl: async () => {
      downloads += 1;
      return new Response(bytes, { status: 200 });
    },
  });

  assert.equal(downloads, 1);
  assert.deepEqual(readFileSync(archive), bytes);
  assert.deepEqual(readFileSync(join(cacheDirectory, spec.archiveName)), bytes);
});
