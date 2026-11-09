import assert from 'node:assert/strict';
import test from 'node:test';
import { teamSlots } from './tactical-teams.ts';

const PLAYERS = [
  { slot: 0, steamid64: '76561198000000000', name: 'a', team_key: 'ct-start', start_side: 'CT' as const },
  { slot: 5, steamid64: '76561198000000005', name: 'b', team_key: 't-start', start_side: 'T' as const },
  { slot: 6, steamid64: '76561198000000006', name: 'c', team_key: 't-start', start_side: 'T' as const },
];

test('a team reads its slots from its SteamIDs, not from the base64 Go sends', () => {
  const team = { slots: JSON.parse('"BQY="'), steamids: ['76561198000000005', '76561198000000006'] };
  assert.deepEqual(teamSlots(team, PLAYERS), [5, 6]);
});

test('a team without SteamIDs keeps slots that are already numbers, and a string never leaks through', () => {
  assert.deepEqual(teamSlots({ slots: [0], steamids: [] }, PLAYERS), [0]);
  assert.deepEqual(teamSlots({ slots: JSON.parse('"AA=="'), steamids: [] }, PLAYERS), []);
});
