import test from 'node:test';
import assert from 'node:assert/strict';
import { pickablePlayers } from './picker-roster.ts';

const line = (name: string, kills: number, deaths: number, assists = 0, adr = 0) => ({ name, kills, deaths, assists, adr });

test('a coach with no kill, death, assist or damage is not offered as a player', () => {
  const roster = [line('ropz', 20, 14, 3, 88), line('Coach|MaseN1WoW', 0, 0), line('support', 0, 0, 2, 0), line('baiter', 0, 1)];
  assert.deepEqual(pickablePlayers(roster).map((p) => p.name), ['ropz', 'support', 'baiter']);
});

test('a player who only dealt damage still played', () => {
  assert.equal(pickablePlayers([line('lurker', 0, 0, 0, 12.5), line('coach', 0, 0)]).length, 1);
});

test('a roster where nobody fought stays whole rather than empty', () => {
  assert.equal(pickablePlayers([line('a', 0, 0), line('b', 0, 0)]).length, 2);
});
