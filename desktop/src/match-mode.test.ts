import { test } from 'node:test';
import assert from 'node:assert/strict';
import { matchModeUrl, parseMatchModeRequest } from './match-mode.ts';

const TEAM = '6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c';

test('a match mode request takes a map and an optional side and team', () => {
  assert.deepEqual(parseMatchModeRequest({ map: 'de_mirage' }), { map: 'de_mirage', side: '', team: '' });
  assert.deepEqual(parseMatchModeRequest({ map: 'de_nuke', side: 'CT', team: TEAM }), { map: 'de_nuke', side: 'CT', team: TEAM });
  assert.deepEqual(parseMatchModeRequest({ map: 'de_nuke', team: 'none' }), { map: 'de_nuke', side: '', team: 'none' });
});

test('a match mode request refuses anything else', () => {
  for (const bad of [null, 'de_mirage', [], {}, { map: 'Mirage!' }, { map: 'de_mirage', side: 'mid' }, { map: 'de_mirage', team: '../x' },
    { map: 'de_mirage', url: 'https://example.com' }]) {
    assert.throws(() => parseMatchModeRequest(bad));
  }
});

test('the match mode URL stays on the web origin', () => {
  const href = matchModeUrl('http://127.0.0.1:3100', { map: 'de_mirage', side: 'T', team: TEAM });
  assert.equal(href, `http://127.0.0.1:3100/libreta/partida?mapa=de_mirage&lado=T&equipo=${TEAM}`);
  assert.equal(matchModeUrl('http://127.0.0.1:3100', { map: 'de_nuke', side: '', team: '' }), 'http://127.0.0.1:3100/libreta/partida?mapa=de_nuke');
});
