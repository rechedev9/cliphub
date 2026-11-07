import assert from 'node:assert/strict';
import test from 'node:test';
import type { Play } from '../api/types.ts';
import { filterPlays, NO_PLAY_FILTER, playFilterOptions } from './play-filter.ts';

function play(id: string, extra: Partial<Play>): Play {
  return { id, matchId: 'm', label: id, kind: 'highlight', round: 1, kills: 1, ...extra };
}

const plays: Play[] = [
  play('awp-ct', { weapons: ['awp'], side: 'CT', kills: 2 }),
  play('mixed-t', { weapons: ['ak47', 'awp'], side: 'T', kills: 3, clutchVs: 2 }),
  play('ace-t', { weapons: ['ak47'], side: 'T', kills: 5 }),
  play('old-plan', { kills: 1 }),
];

function ids(list: Play[]): string[] {
  return list.map((p) => p.id);
}

test('filterPlays keeps every play without a filter', () => {
  assert.deepEqual(ids(filterPlays(plays, NO_PLAY_FILTER)), ['awp-ct', 'mixed-t', 'ace-t', 'old-plan']);
});

test('a weapon keeps only plays where every kill used it', () => {
  assert.deepEqual(ids(filterPlays(plays, { ...NO_PLAY_FILTER, weapon: 'awp' })), ['awp-ct']);
});

test('kinds and sides keep the plays that have them, and combine', () => {
  assert.deepEqual(ids(filterPlays(plays, { ...NO_PLAY_FILTER, kind: 'clutch' })), ['mixed-t']);
  assert.deepEqual(ids(filterPlays(plays, { ...NO_PLAY_FILTER, kind: 'ace' })), ['ace-t']);
  assert.deepEqual(ids(filterPlays(plays, { ...NO_PLAY_FILTER, side: 'T' })), ['mixed-t', 'ace-t']);
  assert.deepEqual(ids(filterPlays(plays, { ...NO_PLAY_FILTER, weapon: 'awp', side: 'T' })), []);
});

test('playFilterOptions offers only what some play has', () => {
  assert.deepEqual(playFilterOptions(plays), { weapons: ['ak47', 'awp'], kinds: ['clutch', 'ace'], sides: ['CT', 'T'], players: [] });
  assert.deepEqual(playFilterOptions([plays[3]]), { weapons: [], kinds: [], sides: [], players: [] });
});

test('a multi-POV match filters by player and offers each player once', () => {
  const ropz = { steamId: '76561198000000001', name: 'ropz' };
  const b1t = { steamId: '76561198000000002', name: 'b1t' };
  const mixed = [play('r1', { player: ropz }), play('b1', { player: b1t }), play('r2', { player: ropz })];
  assert.deepEqual(ids(filterPlays(mixed, { ...NO_PLAY_FILTER, player: ropz.steamId })), ['r1', 'r2']);
  assert.deepEqual(playFilterOptions(mixed).players, [ropz, b1t]);
});
