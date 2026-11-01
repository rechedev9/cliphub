import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { readAgentAccess, removeAgentSession, writeAgentSession } from './agent-session.ts';

function tempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'agent-session-'));
}

const TOKEN = 'a'.repeat(64);

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
  writeAgentSession({ sessionFile: file, enabled: false, orchestratorUrl: 'http://127.0.0.1:1', token: TOKEN });
  assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), { version: 1, enabled: false });
});

test('a session with access on carries the url and token, and removal is idempotent', () => {
  const file = path.join(tempDir(), 'agent-session.json');
  writeAgentSession({ sessionFile: file, enabled: true, orchestratorUrl: 'http://127.0.0.1:4100', token: TOKEN });
  assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), {
    version: 1,
    enabled: true,
    orchestratorUrl: 'http://127.0.0.1:4100',
    token: TOKEN,
  });
  removeAgentSession(file);
  removeAgentSession(file);
  assert.equal(fs.existsSync(file), false);
});
