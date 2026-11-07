import assert from 'node:assert/strict';
import test from 'node:test';
import type { RoleSuggestion } from '../api/roles.ts';
import type { LibraryPlay, PlayPosition } from '../api/stratbook.ts';
import { busiestTeam, cleanRoles, fillFromSuggestion, playRoleRows, rolePlayers, rolesFromPositions, sameRoles, sheetFromSuggestion } from './roles.ts';

function play(over: Partial<LibraryPlay>): LibraryPlay {
  return {
    id: over.title ?? 'x',
    title: 'x',
    team_id: 'vit',
    map: 'de_mirage',
    side: 'T',
    kind: 'execute',
    site: 'A',
    origin: 'manual',
    positions_seconds: 0,
    grenades: [],
    positions: [],
    notes: '',
    examples: [],
    status: 'active',
    tags: [],
    roles: [],
    created_at: '',
    updated_at: '',
    ...over,
  };
}

const ct: RoleSuggestion[] = [
  { steamid64: '1', player: 'ZywOo', role: 'awp', callout: 'TopofMid', rounds: 40 },
  { steamid64: '2', player: 'apEX', role: 'anchor', callout: 'BombsiteB', rounds: 38 },
  { steamid64: '3', player: 'flameZ', role: 'rotator', rounds: 30 },
];

test('sheetFromSuggestion names each role in Spanish and keeps the CT spot as players say it', () => {
  assert.deepEqual(sheetFromSuggestion(ct), [
    { name: 'AWP', steamid64: '1', callout: 'Top of Mid' },
    { name: 'Ancla', steamid64: '2', callout: 'Bombsite B' },
    { name: 'Rotador', steamid64: '3' },
  ]);
  const t: RoleSuggestion[] = [
    { steamid64: '4', player: 'a', role: 'unknown', rounds: 1 },
    { steamid64: '5', player: 'b', role: 'support', rounds: 1 },
  ];
  assert.deepEqual(sheetFromSuggestion(t).map((slot) => slot.name), ['Rifler', 'Apoyo']);
  const six = Array.from({ length: 6 }, (_, i): RoleSuggestion => ({ steamid64: String(i), player: '', role: 'entry', rounds: 1 }));
  assert.equal(sheetFromSuggestion(six).length, 5);
});

test('rolesFromPositions takes one role per player, named after them or numbered, five at most', () => {
  const positions: PlayPosition[] = [
    { player: 'apEX', steamid64: '1', pos: [0, 0, 0], callout: 'BombsiteA' },
    { player: 'apEX', steamid64: '1', pos: [1, 1, 1] },
    { pos: [2, 2, 2] },
    ...Array.from({ length: 5 }, (_, i): PlayPosition => ({ player: `p${i}`, steamid64: String(10 + i), pos: [0, 0, 0] })),
  ];
  const roles = rolesFromPositions({ positions });
  assert.deepEqual(roles.slice(0, 2), [{ name: 'apEX', steamid64: '1', callout: 'Bombsite A' }, { name: 'Rol 2' }]);
  assert.equal(roles.length, 5);
  assert.deepEqual(rolesFromPositions({ positions: [] }), []);
});

test('cleanRoles drops blank rows, trims the rest and asks for a name on a filled row', () => {
  assert.deepEqual(cleanRoles([{ name: ' Entry ', steamid64: '', task: '  primero por rampa ' }, { name: '  ', callout: '' }]), {
    ok: true,
    slots: [{ name: 'Entry', task: 'primero por rampa' }],
  });
  assert.deepEqual(cleanRoles([{ name: '', task: 'humo a CT' }]), { ok: false, error: 'Cada rol necesita un nombre.' });
});

test('sameRoles ignores blank rows and spaces but not a changed player', () => {
  assert.ok(sameRoles([{ name: 'AWP', steamid64: '1' }], [{ name: ' AWP ', steamid64: '1', task: '' }, { name: '' }]));
  assert.equal(sameRoles([{ name: 'AWP', steamid64: '1' }], [{ name: 'AWP', steamid64: '2' }]), false);
});

test('fillFromSuggestion fills only the sides without roles', () => {
  const filled = fillFromSuggestion({ t: [{ name: 'Lurk' }], ct: [{ name: '' }] }, { t: [ct[0]], ct });
  assert.deepEqual(filled.t, [{ name: 'Lurk' }]);
  assert.equal(filled.ct.length, 3);
  assert.deepEqual(filled.filled, ['CT']);
  assert.deepEqual(fillFromSuggestion({ t: [], ct: [] }, { t: [], ct: [] }).filled, []);
});

test('rolePlayers lists the roster by nick, then the play players who are not on it', () => {
  const roster = [
    { steamid64: '2', nick: 'ZywOo', first_played_at: '', last_played_at: '', matches: 3 },
    { steamid64: '1', nick: 'apEX', first_played_at: '', last_played_at: '', matches: 3 },
  ];
  const positions: PlayPosition[] = [
    { player: 'apEX', steamid64: '1', pos: [0, 0, 0] },
    { player: 'stand-in', steamid64: '9', pos: [0, 0, 0] },
    { player: 'no id', pos: [0, 0, 0] },
  ];
  assert.deepEqual(rolePlayers(roster, positions), [
    { steamid64: '1', nick: 'apEX' },
    { steamid64: '2', nick: 'ZywOo' },
    { steamid64: '9', nick: 'stand-in' },
  ]);
});

test('playRoleRows flattens the roles of one map and team and names their players', () => {
  const plays = [
    play({
      title: 'A exec',
      roles: [{ name: 'Entry', steamid64: '1' }, { name: 'Lurk', steamid64: '7' }, { name: 'Apoyo' }],
      positions: [{ player: 'flameZ', steamid64: '7', pos: [0, 0, 0] }],
    }),
    play({ title: 'Nuke', map: 'de_nuke', roles: [{ name: 'AWP' }] }),
    play({ title: 'Otro equipo', team_id: 'g2', roles: [{ name: 'AWP' }] }),
    play({ title: 'Sin equipo', team_id: null, roles: [{ name: 'IGL' }] }),
  ];
  const rows = playRoleRows(plays, { map: 'de_mirage', team: 'vit', names: new Map([['1', 'apEX']]) });
  assert.deepEqual(
    rows.map((row) => [row.key, row.slot.name, row.player]),
    [['A exec:0', 'Entry', 'apEX'], ['A exec:1', 'Lurk', 'flameZ'], ['A exec:2', 'Apoyo', '']],
  );
  assert.deepEqual(playRoleRows(plays, { map: 'de_mirage', team: null }).map((row) => row.play.title), ['Sin equipo']);
});

test('busiestTeam opens on the team with most live plays', () => {
  const plays = [play({ team_id: 'g2' }), play({ team_id: 'vit', title: 'a' }), play({ team_id: 'vit', title: 'b', status: 'archived' }), play({ team_id: 'g2', title: 'c' })];
  assert.equal(busiestTeam(plays), 'g2');
  assert.equal(busiestTeam([play({ team_id: null })]), null);
});
