import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import {
  nextAgentAccess,
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
const VERSION = '5.10.0';

const OFF = { enabled: false, autoApproveStarts: false, autoApproveDeletes: false };
const ACCESS_ON = { enabled: true, autoApproveStarts: false, autoApproveDeletes: false };

test('access is off without a file, with bad JSON, or without enabled true', () => {
  const dir = tempDir();
  const file = path.join(dir, 'agent-access.json');
  assert.deepEqual(readAgentAccess(file), OFF);
  fs.writeFileSync(file, '{not json');
  assert.deepEqual(readAgentAccess(file), OFF);
  fs.writeFileSync(file, '[true]');
  assert.deepEqual(readAgentAccess(file), OFF);
  fs.writeFileSync(file, '{"enabled":"yes","autoApproveStarts":"yes","autoApproveDeletes":1}');
  assert.deepEqual(readAgentAccess(file), OFF);
  fs.writeFileSync(file, '{"enabled":true}');
  assert.deepEqual(readAgentAccess(file), ACCESS_ON);
  fs.writeFileSync(file, '{"enabled":false,"autoApproveStarts":true,"autoApproveDeletes":true}');
  assert.deepEqual(readAgentAccess(file), { enabled: false, autoApproveStarts: true, autoApproveDeletes: true });
});

test('a change keeps the stored automatic approvals it does not name', () => {
  const stored = { enabled: true, autoApproveStarts: true, autoApproveDeletes: true };
  assert.deepEqual(nextAgentAccess(stored, { enabled: false }), { enabled: false, autoApproveStarts: true, autoApproveDeletes: true });
  assert.deepEqual(nextAgentAccess(stored, { enabled: true, autoApproveStarts: false }), { enabled: true, autoApproveStarts: false, autoApproveDeletes: true });
  assert.deepEqual(nextAgentAccess(OFF, { enabled: true, autoApproveDeletes: true }), { enabled: true, autoApproveStarts: false, autoApproveDeletes: true });
});

test('a session with access off carries no token', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  writeAgentSession({ sessionFile: file, ...OFF, autoApproveStarts: true, autoApproveDeletes: true, orchestratorUrl: 'http://127.0.0.1:1', token: TOKEN, webOrigin: WEB, studioVersion: VERSION });
  assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), { version: 1, enabled: false });
});

test('a session with access on carries the url and token, and removal is idempotent', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  writeAgentSession({ sessionFile: file, ...ACCESS_ON, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION });
  assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), {
    version: 1,
    enabled: true,
    orchestratorUrl: 'http://127.0.0.1:4100',
    token: TOKEN,
    webOrigin: WEB,
    studioVersion: VERSION,
    autoApproveStarts: false,
    autoApproveDeletes: false,
  });
  removeAgentSession(file);
  removeAgentSession(file);
  assert.equal(fs.existsSync(file), false);
});

test('changing the switch rewrites a running session and only stores the choice before one exists', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };

  setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running: null });
  assert.deepEqual(readAgentAccess(accessFile), ACCESS_ON);
  assert.equal(fs.existsSync(sessionFile), false);

  setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running });
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), { version: 1, ...ACCESS_ON, ...running });

  setAgentAccess({ accessFile, sessionFile, ...OFF, running });
  assert.deepEqual(readAgentAccess(accessFile), OFF);
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), { version: 1, enabled: false });
});

test('the automatic approvals are stored while access is off and published only while it is on', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  const stored = { enabled: false, autoApproveStarts: true, autoApproveDeletes: true };

  setAgentAccess({ accessFile, sessionFile, ...stored, running });
  assert.deepEqual(JSON.parse(fs.readFileSync(accessFile, 'utf8')), stored);
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), { version: 1, enabled: false });

  setAgentAccess({ accessFile, sessionFile, ...nextAgentAccess(readAgentAccess(accessFile), { enabled: true }), running });
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), {
    version: 1,
    enabled: true,
    ...running,
    autoApproveStarts: true,
    autoApproveDeletes: true,
  });

  setAgentAccess({ accessFile, sessionFile, ...nextAgentAccess(readAgentAccess(accessFile), { enabled: true, autoApproveStarts: false }), running });
  assert.equal(JSON.parse(fs.readFileSync(sessionFile, 'utf8')).autoApproveStarts, false);
  assert.deepEqual(readAgentAccess(accessFile), { enabled: true, autoApproveStarts: false, autoApproveDeletes: true });

  setAgentAccess({ accessFile, sessionFile, ...nextAgentAccess(readAgentAccess(accessFile), { enabled: false }), running });
  assert.deepEqual(JSON.parse(fs.readFileSync(sessionFile, 'utf8')), { version: 1, enabled: false });
  assert.deepEqual(readAgentAccess(accessFile), { enabled: false, autoApproveStarts: false, autoApproveDeletes: true });
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
  writeAgentSession({ sessionFile: file, ...ACCESS_ON, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION, fsOps: ops });
  assert.equal(ops.renames, 3);
  assert.equal(JSON.parse(fs.readFileSync(file, 'utf8')).token, TOKEN);
  assert.equal(fs.existsSync(`${file}.tmp`), false);
});

test('a rename that keeps failing removes the temp file and reports the error', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  const ops = flakyFs({ renameFailures: Infinity });
  assert.throws(
    () => writeAgentSession({ sessionFile: file, ...ACCESS_ON, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION, fsOps: ops }),
    /in use/,
  );
  assert.equal(ops.renames, 5);
  assert.equal(fs.existsSync(`${file}.tmp`), false);
  assert.equal(fs.existsSync(file), false);
});

test('an error that is not a lock is not retried', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  const ops = flakyFs({ renameFailures: Infinity, code: 'ENOENT' });
  assert.throws(() => writeAgentSession({ sessionFile: file, ...ACCESS_ON, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION, fsOps: ops }));
  assert.equal(ops.renames, 1);
});

test('disabling access removes the session when it cannot be rewritten, so no token stays', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running });
  assert.equal(JSON.parse(fs.readFileSync(sessionFile, 'utf8')).token, TOKEN);

  // Only the session file's rename keeps failing.
  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === sessionFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  setAgentAccess({ accessFile, sessionFile, ...OFF, running, fsOps: ops });

  assert.deepEqual(readAgentAccess(accessFile), OFF);
  assert.equal(fs.existsSync(sessionFile), false);
  assert.equal(fs.existsSync(`${sessionFile}.tmp`), false);
});

test('turning off an automatic approval removes the session when it cannot be rewritten, so no approval stays published', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  const granted = { enabled: true, autoApproveStarts: true, autoApproveDeletes: true };
  setAgentAccess({ accessFile, sessionFile, ...granted, running });
  assert.equal(JSON.parse(fs.readFileSync(sessionFile, 'utf8')).autoApproveStarts, true);

  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === sessionFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...granted, autoApproveStarts: false, running, fsOps: ops }), /in use/);

  assert.equal(fs.existsSync(sessionFile), false);
  assert.equal(fs.existsSync(`${sessionFile}.tmp`), false);
  assert.deepEqual(readAgentAccess(accessFile), { ...granted, autoApproveStarts: false });
});

test('turning off an automatic approval stores it even when the session can be neither rewritten nor removed', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  const granted = { enabled: true, autoApproveStarts: true, autoApproveDeletes: true };
  setAgentAccess({ accessFile, sessionFile, ...granted, running });

  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  const realRm = ops.rmSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === sessionFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  ops.rmSync = (file: string, options: { force: true }): void => {
    if (file === sessionFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRm(file, options);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...granted, autoApproveDeletes: false, running, fsOps: ops }), /in use/);

  assert.deepEqual(readAgentAccess(accessFile), { ...granted, autoApproveDeletes: false });
});

test('a grant whose session cannot be published is not stored, so the next boot does not publish it', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running });

  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === sessionFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, autoApproveDeletes: true, running, fsOps: ops }), /in use/);

  assert.deepEqual(readAgentAccess(accessFile), ACCESS_ON);
  assert.equal(fs.existsSync(sessionFile), false);
});

test('a grant that cannot be stored is withdrawn from the published session', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running });

  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  let accessWrites = 0;
  ops.renameSync = (from: string, to: string): void => {
    if (to === accessFile && ++accessWrites > 1) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, autoApproveDeletes: true, running, fsOps: ops }), /in use/);

  assert.deepEqual(readAgentAccess(accessFile), ACCESS_ON);
  const session = JSON.parse(fs.readFileSync(sessionFile, 'utf8'));
  assert.equal(session.token, TOKEN);
  assert.equal(session.autoApproveDeletes, false);
});

test('a revocation that cannot be stored drops the stored choice and the published session', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  const granted = { enabled: true, autoApproveStarts: true, autoApproveDeletes: true };
  setAgentAccess({ accessFile, sessionFile, ...granted, running });

  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === accessFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...granted, autoApproveDeletes: false, running, fsOps: ops }), /in use/);

  assert.deepEqual(readAgentAccess(accessFile), OFF);
  assert.equal(fs.existsSync(sessionFile) && JSON.parse(fs.readFileSync(sessionFile, 'utf8')).token !== undefined, false);
});

test('a revocation whose stored choice can be neither replaced nor removed stays off after a restart until the next stored change', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  const granted = { enabled: true, autoApproveStarts: true, autoApproveDeletes: true };
  setAgentAccess({ accessFile, sessionFile, ...granted, running });

  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  const realRm = ops.rmSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === accessFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRename(from, to);
  };
  ops.rmSync = (file: string, options: { force: true }): void => {
    if (file === accessFile) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    realRm(file, options);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...granted, autoApproveDeletes: false, running, fsOps: ops }), /in use/);
  assert.equal(JSON.parse(fs.readFileSync(accessFile, 'utf8')).autoApproveDeletes, true);

  const boot = readAgentAccess(accessFile);
  assert.deepEqual(boot, OFF);
  writeAgentSession({ sessionFile, ...boot, ...running });
  assert.equal(JSON.parse(fs.readFileSync(sessionFile, 'utf8')).token, undefined);

  const lockedMarker = flakyFs({ renameFailures: 0 });
  const markerRm = lockedMarker.rmSync;
  lockedMarker.rmSync = (file: string, options: { force: true }): void => {
    if (file === `${accessFile}.revoked`) throw Object.assign(new Error('file is in use'), { code: 'EPERM' });
    markerRm(file, options);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running, fsOps: lockedMarker }), /in use/);
  assert.deepEqual(readAgentAccess(accessFile), OFF);
  assert.equal(JSON.parse(fs.readFileSync(sessionFile, 'utf8')).token, undefined);

  setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running });
  assert.equal(fs.existsSync(`${accessFile}.revoked`), false);
  assert.deepEqual(readAgentAccess(accessFile), ACCESS_ON);
});

test('a failure storing the choice never leaves a published token behind', () => {
  const dir = tempDir();
  const accessFile = path.join(dir, 'agent-access.json');
  const sessionFile = path.join(dir, 'agent-session.json');
  const running = { orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN, webOrigin: WEB, studioVersion: VERSION };
  const ops = flakyFs({ renameFailures: 0 });
  const realRename = ops.renameSync;
  ops.renameSync = (from: string, to: string): void => {
    if (to === accessFile) throw Object.assign(new Error('file is in use'), { code: 'EACCES' });
    realRename(from, to);
  };
  assert.throws(() => setAgentAccess({ accessFile, sessionFile, ...ACCESS_ON, running, fsOps: ops }), /in use/);
  assert.equal(fs.existsSync(sessionFile) && JSON.parse(fs.readFileSync(sessionFile, 'utf8')).token !== undefined, false);
  assert.deepEqual(readAgentAccess(accessFile), OFF);
});
