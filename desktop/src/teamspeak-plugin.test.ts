import assert from 'node:assert/strict';
import test from 'node:test';
import { openTeamSpeakPlugin } from './teamspeak-plugin.ts';

test('openTeamSpeakPlugin opens the bundled package and explains each failure', async () => {
  const opened: string[] = [];
  const lines: string[] = [];
  const base = { packagePath: 'C:\\Studio\\bin\\cliphub_teamspeak.ts3_plugin', logLine: (line: string) => lines.push(line) };

  const ok = await openTeamSpeakPlugin({ ...base, exists: () => true, openPath: async (file) => { opened.push(file); return ''; } });
  assert.deepEqual(ok, { ok: true });
  assert.deepEqual(opened, [base.packagePath]);

  const missing = await openTeamSpeakPlugin({ ...base, exists: () => false, openPath: async () => '' });
  assert.equal(missing.ok, false);
  assert.match(missing.ok ? '' : missing.error, /Reinstala Studio/);

  const noApp = await openTeamSpeakPlugin({ ...base, exists: () => true, openPath: async () => 'No application is associated' });
  assert.match(noApp.ok ? '' : noApp.error, /Instala TeamSpeak 3/);

  const thrown = await openTeamSpeakPlugin({ ...base, exists: () => true, openPath: async () => { throw new Error('boom'); } });
  assert.equal(thrown.ok, false);
  assert.equal(lines.length, 3);
});
