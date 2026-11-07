import assert from 'node:assert/strict';
import test from 'node:test';
import type { Play } from '../api/types.ts';
import { DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import { addWithoutClashes, bestMomentsEdit, MAX_POV_PLAYERS, playsClash, povPlayersLabel, togglePovPick } from './multi-pov.ts';

const ropz = { steamId: '76561198000000001', name: 'ropz' };
const b1t = { steamId: '76561198000000002', name: 'b1t' };

function play(id: string, player: Play['player'], startSeconds: number, endSeconds: number): Play {
  return { id, matchId: 'm', label: id, kind: 'highlight', round: 1, kills: 1, player, startSeconds, endSeconds };
}

test('two players clash when their plays are less than half a second apart', () => {
  assert.equal(playsClash(play('a', ropz, 10, 20), play('b', b1t, 15, 25)), true);
  assert.equal(playsClash(play('a', ropz, 10, 20), play('b', b1t, 20.4, 25)), true);
  assert.equal(playsClash(play('a', ropz, 10, 20), play('b', b1t, 20.5, 25)), false);
  assert.equal(playsClash(play('a', ropz, 10, 20), play('b', ropz, 15, 25)), false, 'one POV films both');
  assert.equal(playsClash(play('a', undefined, 10, 20), play('b', b1t, 15, 25)), false, 'single-POV plays never clash');
});

test('addWithoutClashes keeps the chosen plays and adds the rest in order when they fit', () => {
  const kept = [play('a', ropz, 10, 20)];
  const candidates = [play('a', ropz, 10, 20), play('b', b1t, 15, 25), play('c', b1t, 40, 50), play('d', ropz, 45, 55)];
  assert.deepEqual(addWithoutClashes(kept, candidates).map((p) => p.id), ['a', 'c']);
});

test('togglePovPick adds teammates, restarts on the other team and stops at five', () => {
  const t = (n: number) => ({ steamId: `t${n}`, team: 'T' });
  assert.deepEqual(togglePovPick([t(1)], t(2)), [t(1), t(2)]);
  assert.deepEqual(togglePovPick([t(1), t(2)], t(1)), [t(2)]);
  assert.deepEqual(togglePovPick([t(1), t(2)], { steamId: 'ct1', team: 'CT' }), [{ steamId: 'ct1', team: 'CT' }]);
  const full = [1, 2, 3, 4, 5].map(t);
  assert.equal(togglePovPick(full, t(6)).length, MAX_POV_PLAYERS);
});

test('bestMomentsEdit gives the 16:9 look with comms and no single-player counter', () => {
  const edit = bestMomentsEdit({ ...DEFAULT_EDIT_CONFIG, format: 'short-9x16', killCounter: true, centeredKillfeed: true, matchRecap: true });
  assert.equal(edit.format, 'landscape-16x9');
  assert.equal(edit.matchRecap, false);
  assert.equal(edit.nativeHud, true);
  assert.equal(edit.voiceComms, true);
  assert.equal(edit.killCounter, false);
  assert.equal(edit.centeredKillfeed, undefined);
});

test('povPlayersLabel names two players and counts the rest', () => {
  assert.equal(povPlayersLabel(['ropz']), 'ropz');
  assert.equal(povPlayersLabel(['ropz', 'b1t']), 'ropz y b1t');
  assert.equal(povPlayersLabel(['ropz', 'b1t', 'jL', 'm0NESY']), 'ropz, b1t y 2 más');
});
