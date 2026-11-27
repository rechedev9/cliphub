import test from 'node:test';
import assert from 'node:assert/strict';
import type { ProTeamImport } from '../api/pro-matches.ts';
import { proTeamKey, publicOnlyTeams, publicTeamFor, syncPublicTeam } from './public-team.ts';

const catalog = [
  { id: 9565, name: 'Vitality', matches: 12 },
  { id: 6667, name: 'FaZe', matches: 9 },
  { id: 8297, name: 'FURIA', matches: 7 },
  { id: 4608, name: 'Natus Vincere', matches: 5 },
];

test('a team key ignores case, spaces, symbols and a leading Team, as the community server does', () => {
  assert.equal(proTeamKey('Team Vitality'), 'vitality');
  assert.equal(proTeamKey('FaZe Clan'), 'fazeclan');
  assert.equal(proTeamKey('Team'), 'team');
  assert.equal(proTeamKey('Natus Vincere'), 'natusvincere');
});

test('a library team finds its HLTV team by its name or one of its aliases', () => {
  assert.equal(publicTeamFor({ name: 'Team Vitality', aliases: [] }, catalog)?.id, 9565);
  assert.equal(publicTeamFor({ name: 'FaZe Clan', aliases: ['FaZe'] }, catalog)?.id, 6667);
  assert.equal(publicTeamFor({ name: 'NAVI', aliases: [] }, catalog), null);
});

test('the picker adds the public teams the library has no match of, by name', () => {
  const library = [
    { name: 'Vitality', aliases: [], match_count: 4 },
    { name: 'FURIA', aliases: [], match_count: 0 },
    { name: 'NAVI', aliases: ['Natus Vincere'], match_count: 2 },
  ];
  assert.deepEqual(
    publicOnlyTeams(library, catalog).map((team) => team.name),
    ['FaZe', 'FURIA'],
  );
});

function batches(answers: ProTeamImport[]): { call: (id: number) => Promise<ProTeamImport>; calls: () => number } {
  let n = 0;
  return {
    call: async (id) => {
      assert.equal(id, 9565);
      const answer = answers[n];
      n += 1;
      if (answer === undefined) throw new Error('called past the last batch');
      return answer;
    },
    calls: () => n,
  };
}

const batch = (over: Partial<ProTeamImport>): ProTeamImport => ({ name: 'Vitality', days: 120, total: 9, held: 0, imported: 0, remaining: 0, ...over });

test('the sync asks for batches until no map is left, adding up what came in', async () => {
  const fake = batches([
    batch({ held: 4, imported: 4, remaining: 5 }),
    batch({ team_id: 'team-1', held: 8, imported: 4, remaining: 1 }),
    batch({ team_id: 'team-1', held: 8, imported: 0, remaining: 0 }),
  ]);
  const seen: number[] = [];
  const done = await syncPublicTeam({
    hltvId: 9565,
    call: fake.call,
    onBatch: (progress) => {
      seen.push(progress.held);
      return true;
    },
  });
  assert.equal(fake.calls(), 3);
  assert.deepEqual(seen, [4, 8, 8]);
  assert.deepEqual(done, { name: 'Vitality', held: 8, total: 9, imported: 8, teamId: 'team-1' });
});

test('the sync stops when a batch leaves as many maps, or when the caller stops it', async () => {
  const stuck = batches([batch({ remaining: 6 }), batch({ remaining: 6 }), batch({ remaining: 6 })]);
  await syncPublicTeam({ hltvId: 9565, call: stuck.call, onBatch: () => true });
  assert.equal(stuck.calls(), 2);
  const first = batches([batch({ team_id: 'team-1', held: 4, imported: 4, remaining: 5 }), batch({ remaining: 1 })]);
  const progress = await syncPublicTeam({ hltvId: 9565, call: first.call, onBatch: (p) => p.teamId === null });
  assert.equal(first.calls(), 1);
  assert.equal(progress.teamId, 'team-1');
});
