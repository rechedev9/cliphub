import test from 'node:test';
import assert from 'node:assert/strict';
import { libraryUpstreamQuery, matchLibraryRoute, pickLibraryKeys } from './_routes.ts';

const ID = '11111111-1111-4111-8111-111111111111';

test('only whitelisted library routes with valid parameters match', () => {
  assert.ok(matchLibraryRoute('get', ['matches', ID, 'tactical', 'rounds', '12']));
  assert.ok(matchLibraryRoute('PUT', ['matches', ID, 'teams', 't-start']));
  assert.ok(matchLibraryRoute('GET', ['players', '76561198000000001', 'role']));
  assert.equal(matchLibraryRoute('PUT', ['matches', ID, 'teams', 'spectators']), undefined);
  assert.equal(matchLibraryRoute('GET', ['matches', '../jobs']), undefined);
  assert.equal(matchLibraryRoute('GET', ['matches', ID, 'tactical', 'rounds', '0']), undefined);
  assert.equal(matchLibraryRoute('DELETE', ['teams', ID]), undefined);
  assert.equal(matchLibraryRoute('GET', ['series', 'all']), undefined);
  assert.ok(matchLibraryRoute('GET', ['teams', ID, 'scout']));
  assert.ok(matchLibraryRoute('DELETE', ['reports', ID]));
  assert.ok(matchLibraryRoute('GET', ['reports', ID, 'scout']));
  assert.equal(matchLibraryRoute('GET', ['reports', 'all', 'scout']), undefined);
});

test('the upstream query keeps the route names, repeats and order, and drops the rest', () => {
  const route = matchLibraryRoute('GET', ['demos']);
  assert.ok(route);
  const query = libraryUpstreamQuery(route, new URLSearchParams('state=ready&token=x&state=failed&tag=&limit=200'));
  assert.equal(query, 'state=ready&state=failed&limit=200');
});

test('only the listed response keys reach the browser', () => {
  assert.deepEqual(pickLibraryKeys({ id: 'a', secret: 'b', tags: [] }, ['id', 'tags', 'missing']), { id: 'a', tags: [] });
  assert.deepEqual(pickLibraryKeys(['id'], ['id']), {});
  assert.deepEqual(pickLibraryKeys(null, ['id']), {});
});

test('a report patch cannot move the report to another team', () => {
  const route = matchLibraryRoute('PATCH', ['reports', ID]);
  assert.ok(route?.body?.kind === 'json');
  assert.equal(route.body.fields.includes('team_id'), false);
  const scout = matchLibraryRoute('GET', ['teams', ID, 'scout']);
  assert.ok(scout);
  assert.equal(
    libraryUpstreamQuery(scout, new URLSearchParams(`map=de_nuke&match_id=${ID}&side=&token=x&match_id=${ID}`)),
    `map=de_nuke&match_id=${ID}&match_id=${ID}`,
  );
});

test('the replay views and notes are reachable, and a note cannot change match or round', () => {
  assert.ok(matchLibraryRoute('GET', ['matches', ID, 'tactical', 'heatmap']));
  assert.ok(matchLibraryRoute('GET', ['teams', ID, 'overlay']));
  assert.ok(matchLibraryRoute('DELETE', ['annotations', ID]));
  assert.equal(matchLibraryRoute('GET', ['annotations', ID]), undefined);
  const patch = matchLibraryRoute('PATCH', ['annotations', ID]);
  assert.ok(patch?.body?.kind === 'json');
  assert.equal(patch.body.fields.includes('match_id'), false);
  assert.equal(patch.body.fields.includes('round'), false);
  const heat = matchLibraryRoute('GET', ['teams', ID, 'heatmap']);
  assert.ok(heat);
  assert.equal(
    libraryUpstreamQuery(heat, new URLSearchParams('map=de_nuke&kind=timing&slot=3&grenade=smoke&t_to=20&token=x')),
    'map=de_nuke&kind=timing&t_to=20&grenade=smoke',
  );
  const matchHeat = matchLibraryRoute('GET', ['matches', ID, 'tactical', 'heatmap']);
  assert.ok(matchHeat);
  assert.equal(libraryUpstreamQuery(matchHeat, new URLSearchParams('team=ct-start&slot=3&kind=death')), 'team=ct-start&slot=3&kind=death');
});

test('the stratbook routes are reachable and a play patch cannot move its map, side or origin', () => {
  assert.ok(matchLibraryRoute('PATCH', ['collections', ID]));
  assert.equal(matchLibraryRoute('PUT', ['collections', ID]), undefined);
  assert.ok(matchLibraryRoute('GET', ['plays']));
  assert.ok(matchLibraryRoute('GET', ['plays', 'export']));
  assert.ok(matchLibraryRoute('DELETE', ['plays', ID]));
  assert.ok(matchLibraryRoute('GET', ['teams', ID, 'plays']));
  assert.ok(matchLibraryRoute('GET', ['teams', ID, 'grenades']));
  assert.ok(matchLibraryRoute('GET', ['matches', ID, 'tactical', 'rounds', '3', 'play']));
  assert.ok(matchLibraryRoute('GET', ['matches', ID, 'tactical', 'rounds', '3', 'lineups']));
  assert.equal(matchLibraryRoute('DELETE', ['plays', 'export']), undefined);
  assert.equal(matchLibraryRoute('GET', ['matches', ID, 'tactical', 'rounds', '0', 'play']), undefined);
  const patch = matchLibraryRoute('PATCH', ['plays', ID]);
  assert.ok(patch?.body?.kind === 'json');
  for (const field of ['map', 'side', 'origin', 'positions_seconds']) assert.equal(patch.body.fields.includes(field), false);
  const imported = matchLibraryRoute('POST', ['plays', 'import']);
  assert.ok(imported?.body?.kind === 'json');
  assert.equal(imported.body.maxBytes, 8 * 1024 * 1024);
  const draft = matchLibraryRoute('GET', ['matches', ID, 'tactical', 'rounds', '3', 'play']);
  assert.ok(draft);
  assert.equal(libraryUpstreamQuery(draft, new URLSearchParams('team_key=t-start&tick=900&slot=1')), 'team_key=t-start&tick=900');
});

test('the playbook keeps its side and the prep drops it', () => {
  const playbook = matchLibraryRoute('GET', ['teams', ID, 'playbook']);
  const prep = matchLibraryRoute('GET', ['teams', ID, 'prep']);
  assert.ok(playbook && prep);
  assert.equal(matchLibraryRoute('POST', ['teams', ID, 'playbook']), undefined);
  assert.equal(libraryUpstreamQuery(playbook, new URLSearchParams('map=de_nuke&side=T&slot=2')), 'map=de_nuke&side=T');
  assert.equal(libraryUpstreamQuery(prep, new URLSearchParams(`map=de_nuke&side=T&opponent_id=${ID}`)), `map=de_nuke&opponent_id=${ID}`);
});

test('the metrics routes are read only and a player metrics query keeps its own names', () => {
  assert.ok(matchLibraryRoute('GET', ['matches', ID, 'metrics']));
  assert.ok(matchLibraryRoute('GET', ['reports', ID, 'metrics']));
  assert.equal(matchLibraryRoute('POST', ['matches', ID, 'metrics']), undefined);
  assert.equal(matchLibraryRoute('GET', ['players', 'nick', 'metrics']), undefined);
  const player = matchLibraryRoute('GET', ['players', '76561198000000001', 'metrics']);
  assert.ok(player);
  assert.equal(
    libraryUpstreamQuery(player, new URLSearchParams('map=de_nuke&side=T&limit=20&steamid=1&token=x')),
    'map=de_nuke&side=T&limit=20',
  );
});

test('series, map history and the veto simulator are reachable with their own names only', () => {
  for (const method of ['GET', 'PATCH', 'DELETE']) assert.ok(matchLibraryRoute(method, ['series', ID]));
  assert.ok(matchLibraryRoute('POST', ['series']));
  assert.equal(matchLibraryRoute('PUT', ['series', ID]), undefined);
  const series = matchLibraryRoute('PATCH', ['series', ID]);
  assert.ok(series?.body?.kind === 'json');
  assert.deepEqual(series.body.fields, ['format', 'event', 'veto', 'team_ids', 'vetoes']);
  assert.ok(matchLibraryRoute('GET', ['teams', ID, 'maps']));
  const veto = matchLibraryRoute('GET', ['teams', ID, 'veto']);
  assert.ok(veto);
  assert.equal(
    libraryUpstreamQuery(veto, new URLSearchParams(`opponent_id=${ID}&format=bo3&first=a&pool=de_nuke&pool=de_mirage&done=de_nuke&token=x`)),
    `opponent_id=${ID}&format=bo3&first=a&pool=de_nuke&pool=de_mirage&done=de_nuke`,
  );
  const bombsites = matchLibraryRoute('GET', ['teams', ID, 'bombsites']);
  assert.ok(bombsites);
  assert.equal(libraryUpstreamQuery(bombsites, new URLSearchParams('map=de_nuke&side=CT&limit=5')), 'map=de_nuke&side=CT');
  const positions = matchLibraryRoute('GET', ['players', '76561198000000001', 'positions']);
  assert.ok(positions);
  assert.equal(libraryUpstreamQuery(positions, new URLSearchParams('map=de_nuke&side=T&to=x')), 'map=de_nuke&to=x');
  assert.equal(matchLibraryRoute('GET', ['players', 'nick', 'positions']), undefined);
  const job = matchLibraryRoute('POST', ['demos', ID, 'jobs']);
  assert.ok(job?.body?.kind === 'json');
  assert.deepEqual(job.body.fields, ['target_steamid']);
});

test('the baseline and the leaderboard are read only and keep their own query names', () => {
  assert.equal(matchLibraryRoute('POST', ['baseline']), undefined);
  const baseline = matchLibraryRoute('GET', ['baseline']);
  assert.ok(baseline);
  assert.equal(
    libraryUpstreamQuery(baseline, new URLSearchParams('scope=pro&map=de_nuke&metric=kpr&min_rounds=50&token=x')),
    'scope=pro&map=de_nuke&min_rounds=50',
  );
  const board = matchLibraryRoute('GET', ['leaderboard']);
  assert.ok(board);
  assert.equal(
    libraryUpstreamQuery(board, new URLSearchParams('metric=adr&map=de_nuke&min_rounds=80&limit=10&steamid=1')),
    'map=de_nuke&min_rounds=80&metric=adr&limit=10',
  );
});

test('the norms are read only and forward only the map and the dates', () => {
  assert.equal(matchLibraryRoute('POST', ['norms']), undefined);
  const norms = matchLibraryRoute('GET', ['norms']);
  assert.ok(norms);
  assert.equal(
    libraryUpstreamQuery(norms, new URLSearchParams('map=de_mirage&side=CT&from=2026-01-01T00:00:00Z&team_id=x')),
    'map=de_mirage&from=2026-01-01T00%3A00%3A00Z',
  );
  assert.ok(norms.response.kind === 'json');
  assert.ok(norms.response.keys.includes('playbook'));
});

test('share packages export as downloads with only a message, and import as multipart', () => {
  for (const base of ['collections', 'reports']) {
    const route = matchLibraryRoute('POST', [base, ID, 'export']);
    assert.equal(route?.response.kind, 'download');
    assert.ok(route?.body?.kind === 'json');
    assert.deepEqual(route.body.fields, ['notes']);
    assert.equal(matchLibraryRoute('GET', [base, ID, 'export']), undefined);
  }
  const imported = matchLibraryRoute('POST', ['import-package']);
  assert.equal(imported?.body?.kind, 'multipart');
  assert.ok(imported?.response.kind === 'json');
  assert.ok(imported.response.keys.includes('collection_id'));
});
