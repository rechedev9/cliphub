import assert from 'node:assert/strict';
import test from 'node:test';
import type { PlaybookRound, PlaybookTactic, TeamPlaybook } from '../api/playbook.ts';
import {
  EMPTY_COMBO,
  comboFromQuery,
  comboSentence,
  comboSummary,
  writeComboQuery,
  narrowPlaybook,
  playFromCombo,
  positionsNote,
  tacticLabel,
  toggleTactic,
  withoutLabel,
} from './playbook-combo.ts';

const MATCH = '11111111-1111-4111-8111-111111111111';
const RATE = { count: 0, total: 0, pct: 0, reliable: false };

function round(number: number, overrides: Partial<PlaybookRound>): PlaybookRound {
  return {
    match_id: MATCH,
    round: number,
    tick: number * 1000,
    category: 'default',
    pattern: 'default',
    site: 'none',
    won: false,
    buy: 'full',
    opponent_buy: 'full',
    tempo_seconds: null,
    framed: true,
    zones: [],
    tactics: [],
    ...overrides,
  };
}

function tactic(key: string, label?: string): PlaybookTactic {
  const [kind = '', raw = ''] = key.split(':');
  const [grenade, value] = kind === 'utility' ? raw.split('@') : [undefined, raw];
  const out: PlaybookTactic = { key, kind: 'category', value: value ?? raw, label, rounds: 9, share: RATE, win_rate: RATE, refs: [] };
  for (const known of ['category', 'site', 'buy', 'opponent_buy', 'opening', 'opening_result', 'awp', 'zone', 'utility'] as const) {
    if (known === kind) out.kind = known;
  }
  if (grenade === 'smoke' || grenade === 'molotov') out.grenade = grenade;
  return out;
}

const hold = (zone: string): PlaybookRound['zones'][number] => ({ zone, from_seconds: 10, seconds: 6, players: 2 });

const ROUNDS = [
  round(1, { category: 'rush', pattern: 'fast', site: 'B', won: true, tempo_seconds: 10, zones: [hold('TopofMid')],
    tactics: ['category:rush', 'site:B', 'zone:TopofMid', 'utility:smoke@Window'] }),
  round(2, { category: 'execute', pattern: 'execute', site: 'A', tempo_seconds: 30, tactics: ['category:execute', 'site:A', 'utility:smoke@Window'] }),
  round(3, { category: 'execute', pattern: 'execute', site: 'B', won: true, tempo_seconds: 40, zones: [hold('TopofMid')],
    tactics: ['category:execute', 'site:B', 'zone:TopofMid', 'awp:Mid'] }),
  round(4, { category: 'save', pattern: 'save', framed: false, tactics: ['category:save', 'site:none'] }),
];

const TACTICS = ['category:rush', 'category:execute', 'category:save', 'site:A', 'site:B', 'site:none', 'awp:Mid', 'zone:TopofMid', 'utility:smoke@Window'];

const PLAYBOOK: TeamPlaybook = {
  team_id: 't',
  team_name: 'Vitality',
  skipped: [],
  map: 'de_mirage',
  side: 'T',
  thresholds: { rush_seconds: 25, fake_utility: 2, site_contact_players: 2, map_control_players: 2, map_control_seconds: 5, map_control_window_seconds: 40, tempo_bucket_seconds: 5 },
  matches: 1,
  round_count: 4,
  win_rate: RATE,
  framed: 3,
  categories: [],
  tempo: { bucket_seconds: 5, samples: 3, median: 30, buckets: [] },
  sites: (['A', 'B', 'mid'] as const).map((site) => ({ site, rounds: 0, share: RATE, win_rate: RATE, refs: [] })),
  zones: [{ zone: 'TopofMid', label: 'Top of Mid', rounds: 2, share: RATE, win_rate: RATE, refs: [] }],
  tactics: TACTICS.map((key) => tactic(key, key.endsWith('Window') ? 'Ventana' : undefined)),
  combo: { with: [], without: [] },
  rounds: ROUNDS,
};

test('a tactic toggles in and out, and moves between with and without', () => {
  const withSmoke = toggleTactic(EMPTY_COMBO, 'utility:smoke@Window', 'with');
  assert.deepEqual(withSmoke, { with: ['utility:smoke@Window'], without: [] });
  assert.deepEqual(toggleTactic(withSmoke, 'utility:smoke@Window', 'without'), { with: [], without: ['utility:smoke@Window'] });
  assert.deepEqual(toggleTactic(withSmoke, 'utility:smoke@Window', 'with'), EMPTY_COMBO);
});

test('narrowing recounts every block over the rounds the combination keeps', () => {
  const view = narrowPlaybook(PLAYBOOK, { with: ['site:B'], without: ['awp:Mid'] });
  assert.deepEqual(view.rounds.map((r) => r.round), [1]);
  assert.equal(view.round_count, 1);
  assert.equal(view.win_rate.pct, 100);
  assert.deepEqual(view.categories.map((c) => [c.category, c.rounds]), [['save', 0], ['fake', 0], ['rush', 1], ['execute', 0], ['map_control', 0], ['default', 0]]);
  assert.deepEqual(view.zones.map((z) => [z.label, z.rounds, z.share.total]), [['Top of Mid', 1, 1]]);
  assert.deepEqual(view.tactics.map((t) => t.key), ['category:rush', 'site:B', 'zone:TopofMid', 'utility:smoke@Window']);
  assert.equal(view.tempo.median, 10);

  const wide = narrowPlaybook(PLAYBOOK, { with: ['category:execute'], without: [] });
  assert.equal(wide.tactics[0]?.key, 'category:execute');
  assert.equal(wide.tactics.find((t) => t.key === 'site:A')?.share.pct, 50);
  assert.equal(narrowPlaybook(PLAYBOOK, EMPTY_COMBO), PLAYBOOK);
});

test('tactics and combinations read as Spanish phrases', () => {
  const labelOf = (key: string): string => tacticLabel(tactic(key, key.endsWith('Window') ? 'Ventana' : undefined));
  assert.equal(labelOf('utility:molotov@Window'), 'Molotov en Ventana');
  assert.equal(labelOf('opening_result:lost'), 'Primer duelo perdido');
  assert.equal(withoutLabel(labelOf('opening_result:lost')), 'Sin primer duelo perdido');
  assert.equal(withoutLabel(labelOf('awp:Mid')), 'Sin AWP en Mid');
  assert.equal(labelOf('buy:force'), 'Compra force');
  assert.equal(comboSentence({ with: ['category:rush', 'utility:smoke@Window'], without: ['awp:Mid', 'site:A'] }, labelOf), 'Rush + Humo en Ventana, sin AWP en Mid ni ronda en A');
  assert.equal(comboSentence(EMPTY_COMBO, labelOf), '');
});

test('a combination saves as a play with the common pattern and site and its rounds', () => {
  const view = narrowPlaybook(PLAYBOOK, { with: ['site:B'], without: [] });
  const play = playFromCombo(view, { teamId: 't', title: ' Ventana + B ', sentence: 'Ronda en B' });
  assert.equal(play.title, 'Ventana + B');
  assert.equal(play.origin, 'playbook');
  assert.equal(play.site, 'B');
  assert.equal(play.kind, 'fast');
  assert.deepEqual(play.examples.map((e) => e.round), [1, 3]);
  assert.equal(play.notes, 'Combinación del playbook: Ronda en B. Gana el 100 % de 2 rondas.');
  const one = playFromCombo(narrowPlaybook(PLAYBOOK, { with: ['site:A'], without: [] }), { teamId: 't', title: 'A', sentence: 'Ronda en A' });
  assert.equal(one.notes, 'Combinación del playbook: Ronda en A. Gana el 0 % de 1 ronda.');
});

test('a combination of only exclusions starts with a capital', () => {
  assert.equal(comboSentence({ with: [], without: ['awp:Mid'] }, (key) => (key === 'awp:Mid' ? 'AWP en Mid' : key)), 'Sin AWP en Mid');
});

test('the narrowed summary and the positions note count rounds in Spanish', () => {
  assert.equal(comboSummary(narrowPlaybook(PLAYBOOK, { with: ['awp:Mid'], without: [] }), 4), 'Gana el 100 % de 1 ronda de la combinación, de 4.');
  assert.equal(comboSummary(narrowPlaybook(PLAYBOOK, { with: ['site:B', 'site:A'], without: [] }), 4), 'Ninguna de sus 4 rondas cumple la combinación.');
  assert.equal(positionsNote(PLAYBOOK), 'Zonas y AWP leídas en 3 de 4 rondas; las demás no guardan posiciones.');
  assert.equal(positionsNote({ round_count: 2, framed: 0 }).startsWith('Estas partidas no guardan posiciones'), true);
  assert.equal(positionsNote(narrowPlaybook(PLAYBOOK, { with: ['site:B'], without: [] })), '');
});

test('the combination lives in the query beside the scope and drops keys the playbook does not know', () => {
  const known = new Set(['zone:TopofMid', 'awp:Mid']);
  const query = writeComboQuery({ with: ['zone:TopofMid'], without: ['awp:Mid'] }, 'tab=playbook&map=de_mirage&with=zone%3AOld');
  assert.equal(query, 'tab=playbook&map=de_mirage&with=zone%3ATopofMid&without=awp%3AMid');
  assert.deepEqual(comboFromQuery(query, known), { with: ['zone:TopofMid'], without: ['awp:Mid'] });
  assert.deepEqual(comboFromQuery('with=zone%3AOld&with=awp%3AMid&without=awp%3AMid', known), { with: ['awp:Mid'], without: [] });
  assert.equal(writeComboQuery(EMPTY_COMBO, 'tab=playbook&with=awp%3AMid'), 'tab=playbook');
});
