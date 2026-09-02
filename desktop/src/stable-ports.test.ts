import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { allocateStableStudioPort } from './stable-ports.ts';

const TEST_HOST = '127.0.0.1';

function temporaryPortsFile(t: test.TestContext): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-ports-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  return path.join(root, 'ports.json');
}

test('reuses the former web port to preserve the Studio origin', async (t) => {
  const portsFile = temporaryPortsFile(t);
  fs.writeFileSync(portsFile, JSON.stringify({ web: 42002, keep: true }));
  const probes: number[] = [];
  const port = await allocateStableStudioPort({
    host: TEST_HOST, portsFile, logLine: () => {},
    isPortFree: async (candidate) => { probes.push(candidate); return true; },
    allocateFreePort: async () => { throw new Error('unexpected allocation'); },
  });
  assert.equal(port, 42002);
  assert.deepEqual(probes, [42002]);
});

test('retires two-process fields while preserving unknown settings', async (t) => {
  const portsFile = temporaryPortsFile(t);
  fs.writeFileSync(portsFile, JSON.stringify({
    orchestrator: 41001, web: 42002, discovery_secret: 'retired', keep: true,
  }));
  const port = await allocateStableStudioPort({
    host: TEST_HOST, portsFile, logLine: () => {}, isPortFree: async () => true,
  });
  assert.equal(port, 42002);
  assert.deepEqual(JSON.parse(fs.readFileSync(portsFile, 'utf8')), { web: 42002, keep: true });
});

test('allocates and persists one port when no saved origin exists', async (t) => {
  const portsFile = temporaryPortsFile(t);
  const port = await allocateStableStudioPort({
    host: TEST_HOST, portsFile, logLine: () => {}, allocateFreePort: async () => 43003,
  });
  assert.equal(port, 43003);
  assert.deepEqual(JSON.parse(fs.readFileSync(portsFile, 'utf8')), { web: 43003 });
});

test('replaces an occupied saved port and reports the origin change', async (t) => {
  const portsFile = temporaryPortsFile(t);
  fs.writeFileSync(portsFile, JSON.stringify({ web: 42002 }));
  const logs: string[] = [];
  const port = await allocateStableStudioPort({
    host: TEST_HOST, portsFile, logLine: (line) => logs.push(line),
    isPortFree: async () => false, allocateFreePort: async () => 43003,
  });
  assert.equal(port, 43003);
  assert.match(logs[0] ?? '', /localStorage is keyed by origin/);
  assert.deepEqual(JSON.parse(fs.readFileSync(portsFile, 'utf8')), { web: 43003 });
});

test('stops before persistence when allocation is cancelled', async (t) => {
  const portsFile = temporaryPortsFile(t);
  const controller = new AbortController();
  await assert.rejects(allocateStableStudioPort({
    host: TEST_HOST, portsFile, signal: controller.signal, logLine: () => {},
    allocateFreePort: async () => { controller.abort(); return 43003; },
  }), /port allocation aborted/);
  assert.equal(fs.existsSync(portsFile), false);
});
