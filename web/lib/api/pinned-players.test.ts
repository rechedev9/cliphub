import assert from 'node:assert/strict';
import test from 'node:test';
import { savablePins } from './pinned-players.ts';

test('a pin whose id is not a SteamID64 is left out of a save', () => {
  const good = { steamid64: '76561198000000001', name: 'donk' };
  assert.deepEqual(savablePins([good, { steamid64: '5', name: 'five' }, { steamid64: 'x', name: 'bad' }]), [good]);
});
