import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import {
  readAgentAccess,
  removeAgentSession,
  setAgentAccess,
  writeAgentSession,
  type AgentSessionFs,
} from './agent-session.ts';

function tempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'agent-session-'));
}

const TOKEN = 'a'.repeat(64);
const WEB = 'http://127.0.0.1:4200';

test('access is off without a file, with bad JSON, or without enabled true', () => {
  const dir = tempDir();
  const file = path.join(dir, 'agent-access.json');
  assert.equal(readAgentAccess(file), false);
  fs.writeFileSync(file, '{not json');
  assert.equal(readAgentAccess(file), false);
  fs.writeFileSync(file, '{"enabled":"yes"}');
  assert.equal(readAgentAccess(file), false);
  fs.writeFileSync(file, '{"enabled":true}');
  assert.equal(readAgentAccess(file), true);
});

test('a session with access off carries no token', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  writeAgentSession({ sessionFile: file, enabled: false, orchestratorUrl: 'http://127.0.0.1:1', token: TOKEN, webOrigin: WEB });
  assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), { version: 1, enabled: false });
});

test('a session with access on carries the url and token, and removal is idempotent', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  writeAgentSession({ sessionFile: file, enabled: true, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB });
  assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), {
    version: 1,
    enabled: true,
    orchestratorUrl: 'http://127.0.0.1:4100',
    token: TOKEN,
    webOrigin: WEB,
  });
  removeAgentSession(file);
  removeAgentSession(file);
  assert.equal(fs.existsSync(file), false);
});

test('changing the switch rewrites a running session and only stores the choice before one exists', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB };

  setAgentAccess({ accessFile, sessionFile, enabled: true, running: null });
  assert.equal(readAgentAccess(accessFile), true);
  assert.equal(fs.existsSync(sessionFile), false);

  setAgentAccess({ accessFile, sessionFile, enabled: true, running });
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), { version: 1, enabled: true, ...running });

  setAgentAccess({ accessFile, sessionFile, enabled: false, running });
  assert.equal(readAgentAccess(accessFile), false);
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), { version: 1, enabled: false });
});

interface FlakyFsOptions {
  /** Rename attempts that fail before one succeeds; Infinity never succeeds. */
  renameFailures: number;
  code?: string;
}

function flakyFs({ renameFailures, code = 'EPERM' }: FlakyFsOptions): AgentSessionFs & { renames: number } {
  const ops = {
    renames: 0,
    writeFileSync(file: string, data: string, options: { mode: number }): void {
      fs.writeFileSync(file, data, options);
    },
    renameSync(from: string, to: string): void {
      ops.renames += 1;
      if (ops.renames <= renameFailures) throw Object.assign(new Error('file is in use'), { code });
      fs.renameSync(from, to);
    },
    rmSync: (file: string, options: { force: true }): void => fs.rmSync(file, options),
    sleepSync: (): void => {},
  };
  return ops;
}

test('a rename blocked by another process is retried and leaves no temp file', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  const ops = flakyFs({ renameFailures: 2 });
  writeAgentSession({ sessionFile: file, enabled: true, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, fsOps: ops });
  assert.equal(ops.renames, 3);
  assert.equal(JSON.parse(fs.readFileSync(file, 'utf8')).token, TOKEN);
  assert.equal(fs.existsSync(`${file}.tmp`), false);
});

test('a rename that keeps failing removes the temp file and reports the error', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  const ops = flakyFs({ renameFailures: Infinity });
  assert.throws(
    () => writeAgentSession({ sessionFile: file, enabled: true, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, fsOps: ops }),
    /in use/,
  );
  assert.equal(ops.renames, 5);
  assert.equal(fs.existsSync(`${file}.tmp`), false);
  assert.equal(fs.existsSync(file), false);
});

test('an error that is not a lock is not retried', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  const ops = flakyFs({ renameFailures: Infinity, code: 'ENOENT' });
  assert.throws(() => writeAgentSession({ sessionFile: file, enabled: true, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, fsOps: ops }));
  assert.equal(ops.renames, 1);
});

test('disabling access removes the session when it cannot be rewritten, so no token stays', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB };
  setAgentAccess({ accessFile, sessionFile, enabled: true, running });
  assert.equal(JSON.parse(fs.readFileSync(sessionFile, 'utf8')).token, TOKEN);

  // Only the session file's rename keeps failing.
  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === sessionFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  setAgentAccess({ accessFile, sessionFile, enabled: false, running, fsOps: ops });

  assert.equal(readAgentAccess(accessFile), false);
  assert.equal(fs.existsSync(sessionFile), false);
  assert.equal(fs.existsSync(`${sessionFile}.tmp`), false);
});

test('a failure storing the choice never leaves a published token behind', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB };
  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === accessFile) throw Object.assign(new Error('file is in use'), { code: 'EACCES' });
    realRename(from, to);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, enabled: true, running, fsOps: ops }), /in use/);
  assert.equal(fs.existsSync(sessionFile), false);
  assert.equal(readAgentAccess(accessFile), false);
});
