import test from 'node:test';
import assert from 'node:assert/strict';
import { SIDEBAR_FOOTER_ENTRIES, SIDEBAR_GROUPS, VOICE_HREF, activeEntryId, type SidebarEntry } from './nav-groups.ts';
import { NAV_SECTIONS } from './nav.ts';
import { hubTeamHref, scopeTeamId, sidebarHref, withTeam } from './nav-scope.ts';

const ENTRIES = [...SIDEBAR_GROUPS.flatMap((group) => group.entries), ...SIDEBAR_FOOTER_ENTRIES];
const TEAM = '3f2b1c9e-0a4d-4e57-8a61-5d1f7c0b2a11';

function entry(id: SidebarEntry['id']): SidebarEntry {
  const found = ENTRIES.find((candidate) => candidate.id === id);
  assert.ok(found, id);
  return found;
}

test('rail: every entry wears its section name and opens a real destination', () => {
  for (const railed of ENTRIES) {
    const section = NAV_SECTIONS.find((candidate) => candidate.href === railed.href);
    assert.ok(section, railed.id);
    assert.equal(railed.label, section.label, railed.id);
  }
  assert.deepEqual(ENTRIES.map((railed) => railed.href), NAV_SECTIONS.map((section) => section.href));
  assert.equal(entry('replay').label, 'Replay 2D');
  assert.equal(entry('equipos').label, 'Equipos');
  assert.equal(entry('jugadores').label, 'Jugadores');
});

test('rail: Libreta and Informes have their own rows, and one Equipos row stands for the team tools', () => {
  assert.equal(entry('libreta').href, '/libreta');
  assert.equal(entry('informes').href, '/informes');
  assert.equal(ENTRIES.filter((railed) => railed.href.startsWith('/equipos')).length, 1);
  const teamGroup = SIDEBAR_GROUPS.find((group) => group.entries.some((railed) => railed.id === 'jugadores'));
  assert.ok(teamGroup?.entries.some((railed) => railed.id === 'jugadores-faceit'));
});

test('rail: each page lights exactly one entry', () => {
  const at = (pathname: string) => activeEntryId({ pathname });
  assert.equal(at('/replay'), 'replay');
  assert.equal(at('/replay/playlists'), 'partidas');
  assert.equal(at('/patrones'), 'buscador-de-patrones');
  assert.equal(at(`/patrones/${TEAM}`), 'buscador-de-patrones');
  assert.equal(at('/tacticas/nueva'), 'editor-de-tacticas');
  assert.equal(at('/equipos'), 'equipos');
  assert.equal(at(`/equipos/${TEAM}`), 'equipos');
  assert.equal(at('/jugadores'), 'jugadores');
  assert.equal(at('/jugadores/76561198000000000'), 'jugadores');
  assert.equal(at('/jugadores/faceit'), 'jugadores-faceit');
  assert.equal(at('/partidas/series'), 'partidas');
  assert.equal(at('/libreta'), 'libreta');
  assert.equal(at('/libreta/lineups/abc'), 'libreta');
  assert.equal(at('/informes/prep'), 'informes');
  assert.equal(at('/lineups/de_nuke'), 'lineups-pro');
  assert.equal(at('/series/abc'), 'short');
  assert.equal(at('/settings'), 'ajustes');
  assert.equal(at('/nope'), null);
});

test('voice button: opens the Voz del equipo section of Ajustes', () => {
  assert.equal(VOICE_HREF, '/settings#voz-del-equipo');
});

test('scope carry: the team comes from the record path or from `equipo`', () => {
  assert.equal(scopeTeamId({ pathname: `/equipos/${TEAM}`, search: '' }), TEAM);
  assert.equal(scopeTeamId({ pathname: '/replay', search: `equipo=${TEAM}&map=de_nuke` }), TEAM);
  assert.equal(scopeTeamId({ pathname: '/replay', search: 'equipo=' }), null);
  assert.equal(scopeTeamId({ pathname: '/partidas', search: 'map=de_nuke' }), null);
});

test('scope carry: the four team tools keep the team, the map and the rest of the scope', () => {
  const where = { pathname: `/equipos/${TEAM}`, search: `tab=estadisticas&map=de_nuke&side=T&limit=5&buy=full&with=zone%3AMid` };
  const carried = (id: SidebarEntry['id']): URL => new URL(sidebarHref(entry(id), where), 'http://studio.local');

  const replay = carried('replay');
  assert.equal(replay.pathname, '/replay');
  assert.deepEqual([...replay.searchParams], [['equipo', TEAM], ['map', 'de_nuke'], ['side', 'T'], ['limit', '5'], ['buy', 'full']]);

  assert.equal(carried('buscador-de-patrones').pathname, `/patrones/${TEAM}`);
  assert.equal(carried('buscador-de-patrones').searchParams.get('map'), 'de_nuke');
  assert.equal(scopeTeamId({ pathname: `/patrones/${TEAM}`, search: '' }), TEAM);

  const tactics = carried('editor-de-tacticas');
  assert.equal(tactics.pathname, '/tacticas/nueva');
  assert.equal(tactics.searchParams.get('equipo'), TEAM);

  const stats = carried('equipos');
  assert.equal(stats.pathname, `/equipos/${TEAM}`);
  assert.equal(stats.searchParams.get('tab'), null);
  assert.equal(stats.searchParams.get('map'), 'de_nuke');
});

test('scope carry: with no team in scope an entry opens its own door, and the others never change', () => {
  const where = { pathname: '/partidas', search: 'map=de_nuke' };
  assert.equal(sidebarHref(entry('replay'), where), '/replay');
  assert.equal(sidebarHref(entry('equipos'), where), '/equipos');
  assert.equal(sidebarHref(entry('libreta'), where), '/libreta');
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
