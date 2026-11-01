import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { mcpCopyNeeded, syncStableMcp } from './stable-mcp.ts';

function layout(): { source: string; target: string } {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stable-mcp-'));
  const source = path.join(dir, 'bin', 'zv-mcp.exe');
  fs.mkdirSync(path.dirname(source));
  fs.writeFileSync(source, 'version one');
  return { source, target: path.join(dir, 'userData', 'mcp', 'zv-mcp.exe') };
}

test('the copy is needed when missing, resized or from another build', () => {
  const stamp = { size: 100, mtimeMs: 1_700_000_000_000 };
  assert.equal(mcpCopyNeeded({ source: stamp, target: null }), true);
  assert.equal(mcpCopyNeeded({ source: stamp, target: { ...stamp, size: 99 } }), true);
  assert.equal(mcpCopyNeeded({ source: stamp, target: { ...stamp, mtimeMs: stamp.mtimeMs + 60_000 } }), true);
  assert.equal(mcpCopyNeeded({ source: stamp, target: { ...stamp, mtimeMs: stamp.mtimeMs + 500 } }), false);
});

test('copies once, then leaves an identical copy alone', () => {
  const { source, target } = layout();
  assert.deepEqual(syncStableMcp({ source, target }), { path: target, problem: null });
  assert.equal(fs.readFileSync(target, 'utf8'), 'version one');
  const before = fs.statSync(target).ino;
  assert.equal(syncStableMcp({ source, target }).problem, null);
  assert.equal(fs.statSync(target).ino, before);
});

test('replaces the copy when the bundled build changes', () => {
  const { source, target } = layout();
  syncStableMcp({ source, target });
  fs.writeFileSync(source, 'version two, longer');
  assert.equal(syncStableMcp({ source, target }).problem, null);
  assert.equal(fs.readFileSync(target, 'utf8'), 'version two, longer');
});

test('keeps the old copy and cleans up when the copy is in use', () => {
  const { source, target } = layout();
  syncStableMcp({ source, target });
  fs.writeFileSync(source, 'version two, longer');
  const result = syncStableMcp({
    source,
    target,
    renameFile: () => { throw Object.assign(new Error('busy'), { code: 'EBUSY' }); },
  });
  assert.equal(result.path, target);
  assert.match(result.problem ?? '', /EBUSY|busy/);
  assert.equal(fs.readFileSync(target, 'utf8'), 'version one');
  assert.equal(fs.existsSync(`${target}.new`), false);
});

test('falls back to the bundled path when no copy exists and the copy fails', () => {
  const { source, target } = layout();
  const result = syncStableMcp({
    source,
    target,
    renameFile: () => { throw new Error('denied'); },
  });
  assert.equal(result.path, source);
  assert.notEqual(result.problem, null);
});
