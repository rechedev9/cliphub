import test from 'node:test';
import assert from 'node:assert/strict';
import { SIDEBAR_FOOTER_ENTRIES, SIDEBAR_GROUPS, activeEntryId, type SidebarEntry } from './nav-groups.ts';
import { hubTeamHref, scopeTeamId, sidebarHref, withTeam } from './nav-scope.ts';

const ENTRIES = [...SIDEBAR_GROUPS.flatMap((group) => group.entries), ...SIDEBAR_FOOTER_ENTRIES];
const TEAM = '3f2b1c9e-0a4d-4e57-8a61-5d1f7c0b2a11';

function entry(id: SidebarEntry['id']): SidebarEntry {
  const found = ENTRIES.find((candidate) => candidate.id === id);
  assert.ok(found, id);
  return found;
}

test('rail: the analysis side holds EDGE\'s eleven tools, and every entry has a real destination', () => {
  const analysis = SIDEBAR_GROUPS.filter((group) => group.id !== 'crear').flatMap((group) => group.entries);
  assert.equal(analysis.length, 11);
  assert.equal(entry('buscador-de-patrones').href, '/patrones');
  assert.equal(entry('editor-de-tacticas').href, '/tacticas/nueva');
  assert.equal(entry('estadisticas-de-equipo').href, '/equipos');
  assert.equal(entry('estadisticas-de-jugador').href, '/jugadores');
});

test('rail: each page lights exactly one entry, the tools inside a section light their own row', () => {
  const at = (pathname: string, tab: string | null = null) => activeEntryId({ pathname, tab });
  assert.equal(at('/replay'), 'replay');
  assert.equal(at('/replay/playlists'), 'partidas');
  assert.equal(at('/patrones'), 'buscador-de-patrones');
  assert.equal(at('/tacticas/nueva'), 'editor-de-tacticas');
  assert.equal(at('/equipos'), 'estadisticas-de-equipo');
  assert.equal(at(`/equipos/${TEAM}`), 'estadisticas-de-equipo');
  assert.equal(at(`/equipos/${TEAM}`, 'replay'), 'estadisticas-de-equipo');
  assert.equal(at(`/equipos/${TEAM}`, 'playbook'), 'playbook');
  assert.equal(at(`/equipos/${TEAM}`, 'preparar'), 'preparar-rival');
  assert.equal(at(`/equipos/${TEAM}`, 'scouting'), 'preparar-rival');
  assert.equal(at(`/equipos/${TEAM}`, 'veto'), 'simulacion-de-veto');
  assert.equal(at('/jugadores'), 'estadisticas-de-jugador');
  assert.equal(at('/jugadores/76561198000000000'), 'estadisticas-de-jugador');
  assert.equal(at('/jugadores/faceit'), 'jugadores-faceit');
  assert.equal(at('/partidas/series'), 'partidas');
  assert.equal(at('/libreta'), 'playbook');
  assert.equal(at('/informes/prep'), 'preparar-rival');
  assert.equal(at('/integridad'), 'partidas');
  assert.equal(at('/lineups/de_nuke'), 'lineups-pro');
  assert.equal(at('/settings'), 'ajustes');
  assert.equal(at('/nope'), null);
});

test('scope carry: the team comes from the record path or from `equipo`', () => {
  assert.equal(scopeTeamId({ pathname: `/equipos/${TEAM}`, search: '' }), TEAM);
  assert.equal(scopeTeamId({ pathname: '/replay', search: `equipo=${TEAM}&map=de_nuke` }), TEAM);
  assert.equal(scopeTeamId({ pathname: '/replay', search: 'equipo=' }), null);
  assert.equal(scopeTeamId({ pathname: '/partidas', search: 'map=de_nuke' }), null);
});

test('scope carry: the five team tools keep the team, the map and the rest of the scope', () => {
  const where = { pathname: `/equipos/${TEAM}`, search: `tab=estadisticas&map=de_nuke&side=T&limit=5&buy=full&with=zone%3AMid` };
  const carried = (id: SidebarEntry['id']): URL => new URL(sidebarHref(entry(id), where), 'http://studio.local');

  const replay = carried('replay');
  assert.equal(replay.pathname, '/replay');
  assert.deepEqual([...replay.searchParams], [['equipo', TEAM], ['map', 'de_nuke'], ['side', 'T'], ['limit', '5'], ['buy', 'full']]);

  assert.equal(carried('buscador-de-patrones').pathname, `/patrones/${TEAM}`);
  assert.equal(carried('buscador-de-patrones').searchParams.get('map'), 'de_nuke');
  assert.equal(scopeTeamId({ pathname: `/patrones/${TEAM}`, search: '' }), TEAM);

  const stats = carried('estadisticas-de-equipo');
  assert.equal(stats.pathname, `/equipos/${TEAM}`);
  assert.equal(stats.searchParams.get('tab'), null);
  assert.equal(stats.searchParams.get('map'), 'de_nuke');

  const playbook = carried('playbook');
  assert.equal(playbook.searchParams.get('tab'), 'playbook');
  assert.equal(playbook.searchParams.get('side'), 'T');
  assert.equal(playbook.searchParams.get('with'), null);

  assert.equal(carried('preparar-rival').searchParams.get('tab'), 'preparar');
  assert.equal(carried('simulacion-de-veto').searchParams.get('tab'), 'veto');
});

test('scope carry: with no team in scope an entry opens its own door, and the others never change', () => {
  const where = { pathname: '/partidas', search: 'map=de_nuke' };
  assert.equal(sidebarHref(entry('replay'), where), '/replay');
  assert.equal(sidebarHref(entry('playbook'), where), '/equipos?tab=playbook');
  assert.equal(sidebarHref(entry('preparar-rival'), where), '/equipos?tab=preparar');
  const onTeam = { pathname: `/equipos/${TEAM}`, search: 'map=de_nuke' };
  assert.equal(sidebarHref(entry('ranking'), onTeam), '/ranking');
  assert.equal(sidebarHref(entry('lineups-pro'), onTeam), '/lineups');
});

test('scope carry: switching team keeps the scope but drops the rival and the lineup of the old one', () => {
  const href = withTeam({ pathname: '/replay', search: `equipo=old&map=de_nuke&opponent=${TEAM}&lineup=all&side=T` }, 'new');
  const url = new URL(href, 'http://studio.local');
  assert.equal(url.pathname, '/replay');
  assert.deepEqual([...url.searchParams], [['equipo', 'new'], ['map', 'de_nuke'], ['side', 'T']]);
});

test('scope carry: the hub opens a team on the tab the rail asked for', () => {
  assert.equal(hubTeamHref(TEAM, 'playbook'), `/equipos/${TEAM}?tab=playbook`);
  assert.equal(hubTeamHref(TEAM, 'veto'), `/equipos/${TEAM}?tab=veto`);
  assert.equal(hubTeamHref(TEAM, 'x'), `/equipos/${TEAM}`);
  assert.equal(hubTeamHref(TEAM, null), `/equipos/${TEAM}`);
});
