import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { copyAndHash } from './copy-and-hash.ts';

function fixture(t: test.TestContext): { source: string; destination: string } {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-copy-hash-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return { source: path.join(directory, 'source'), destination: path.join(directory, 'destination') };
}

for (const size of [0, 17, 4 * 1024 * 1024 + 7]) {
  test(`copyAndHash preserves ${size} bytes and SHA256 across stream boundaries`, async (t) => {
    const { source, destination } = fixture(t);
    const bytes = Buffer.allocUnsafe(size);
    for (let i = 0; i < size; i++) bytes[i] = i % 251;
    fs.writeFileSync(source, bytes);
    const digest = await copyAndHash(source, destination, new AbortController().signal);
    assert.equal(digest, createHash('sha256').update(bytes).digest('hex'));
    assert.deepEqual(fs.readFileSync(destination), bytes);
  });
}

test('copyAndHash yields to the main event loop on a multi-megabyte copy', async (t) => {
  const { source, destination } = fixture(t);
  fs.writeFileSync(source, Buffer.allocUnsafe(4 * 1024 * 1024 + 7));
  let yielded = false;
  setImmediate(() => { yielded = true; });
  await copyAndHash(source, destination, new AbortController().signal);
  assert.equal(yielded, true, 'copy yields to the main event loop');
});

test('copyAndHash aborts an in-flight copy and closes both file handles', async (t) => {
  const { source, destination } = fixture(t);
  fs.writeFileSync(source, Buffer.alloc(8 * 1024 * 1024));
  const controller = new AbortController();
  const copying = copyAndHash(source, destination, controller.signal);
  controller.abort();
  await assert.rejects(copying, { name: 'AbortError' });
  fs.rmSync(source);
  fs.rmSync(destination, { force: true });
});

test('copyAndHash rejects an already-aborted signal without locking the source', async (t) => {
  const { source, destination } = fixture(t);
  fs.writeFileSync(source, 'archive');
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(copyAndHash(source, destination, controller.signal), { name: 'AbortError' });
  fs.rmSync(source);
  fs.rmSync(destination, { force: true });
});

test('copyAndHash propagates missing-source and destination errors', async (t) => {
  const { source, destination } = fixture(t);
  await assert.rejects(copyAndHash(source, destination, new AbortController().signal), { code: 'ENOENT' });
  fs.writeFileSync(source, 'archive');
  await assert.rejects(copyAndHash(source, path.join(destination, 'missing', 'file'), new AbortController().signal));
});
