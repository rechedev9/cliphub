import { test } from 'node:test';
import assert from 'node:assert/strict';
import { matchModeOpener } from './match-mode.ts';

test('the Modo partida bridge is found only where the desktop exposes it', async () => {
  assert.equal(matchModeOpener({}), null);
  assert.equal(matchModeOpener({ cliphubMatchMode: { open: 'no' } }), null);
  const sent: unknown[] = [];
  const open = matchModeOpener({ cliphubMatchMode: { open: async (request: unknown) => { sent.push(request); return true; } } });
  assert.ok(open);
  assert.equal(await open({ map: 'de_mirage', side: 'CT', team: '' }), true);
  assert.deepEqual(sent, [{ map: 'de_mirage', side: 'CT', team: '' }]);
});
