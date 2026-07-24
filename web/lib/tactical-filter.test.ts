// Unit tests for the tactical filter: the URL is the filter's storage, so a
// filtered workspace must survive a reload and a paste into a teammate's
// browser, and the client-side round predicate must agree with the Go
// `Filter.Match` the aggregate endpoint uses. A drift between the two would show
// a round list whose tendencies were computed from a different set of rounds.
// Run: node --test "lib/**/*.test.ts"
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  TACTICAL_BUY_TYPES,
  TACTICAL_CT_PATTERNS,
  TACTICAL_OUTCOMES,
  TACTICAL_PHASES,
  TACTICAL_SIDES,
  TACTICAL_SITES,
  TACTICAL_T_PATTERNS,
} from './api/tactical.ts';
import type { TacticalFilter, TacticalRound, TacticalSide } from './api/tactical.ts';
import {
  filterTacticalRounds,
  isEmptyTacticalFilter,
  roundMatchesFilter,
  tacticalFilterCount,
  tacticalFilterFromQuery,
  tacticalFilterToQuery,
  tacticalPerspective,
  teamSideInRound,
} from './tactical-filter.ts';

const TEAMS = [
  { key: 'home', start_side: TACTICAL_SIDES.ct },
  { key: 'away', start_side: TACTICAL_SIDES.t },
];

type RoundOverrides = {
  number?: number;
  half?: number;
  overtime?: number;
  winner?: TacticalSide | '';
  ctBuy?: TacticalRound['economy']['ct_buy'];
  tBuy?: TacticalRound['economy']['t_buy'];
  site?: TacticalRound['class']['site'];
  tSide?: TacticalRound['class']['t_side'];
  ctSide?: TacticalRound['class']['ct_side'];
  tags?: string[];
  players?: Array<{ slot: number; side: TacticalSide }>;
};

function round(overrides: RoundOverrides = {}): TacticalRound {
  const number = overrides.number ?? 1;
  const built: TacticalRound = {
    number,
    tick_start: number * 10000,
    tick_freeze_end: number * 10000 + 1280,
    tick_end: number * 10000 + 8000,
    tick_official: number * 10000 + 8320,
    score_ct_before: 0,
    score_t_before: 0,
    winner: overrides.winner ?? TACTICAL_SIDES.ct,
    end_reason: 'ct_win',
    half: overrides.half ?? 1,
    economy: {
      ct_equip_value: 4500,
      t_equip_value: 4000,
      ct_money: 1000,
      t_money: 1000,
      ct_buy: overrides.ctBuy ?? TACTICAL_BUY_TYPES.full,
      t_buy: overrides.tBuy ?? TACTICAL_BUY_TYPES.full,
      sample_tick: number * 10000 + 1728,
    },
    class: {
      t_side: overrides.tSide ?? TACTICAL_T_PATTERNS.default,
      ct_side: overrides.ctSide ?? TACTICAL_CT_PATTERNS.hold,
      site: overrides.site ?? TACTICAL_SITES.a,
      opening_traded: false,
      tags: overrides.tags ?? [],
      reasons: [],
    },
    players: (overrides.players ?? [
      { slot: 0, side: TACTICAL_SIDES.ct },
      { slot: 5, side: TACTICAL_SIDES.t },
    ]).map((player) => ({
      slot: player.slot,
      side: player.side,
      kills: 0,
      deaths: 0,
      assists: 0,
      damage: 0,
      equip_value: 0,
      money: 0,
      survived: true,
    })),
    events: [],
  };
  if (overrides.overtime !== undefined) built.overtime = overrides.overtime;
  return built;
}

test('filter: a full filter survives a round trip through the query string', () => {
  const filter: TacticalFilter = {
    team_key: 'home',
    side: TACTICAL_SIDES.t,
    buys: [TACTICAL_BUY_TYPES.eco, TACTICAL_BUY_TYPES.force],
    opponent_buys: [TACTICAL_BUY_TYPES.full],
    sites: [TACTICAL_SITES.a, TACTICAL_SITES.mid],
    outcome: TACTICAL_OUTCOMES.win,
    t_patterns: [TACTICAL_T_PATTERNS.execute],
    ct_patterns: [TACTICAL_CT_PATTERNS.retake, TACTICAL_CT_PATTERNS.stack],
    tags: ['postplant'],
    slots: [1, 4],
    round_from: 3,
    round_to: 18,
    phase: TACTICAL_PHASES.regulation,
  };

  const query = new URLSearchParams(tacticalFilterToQuery(filter));
  assert.deepEqual(tacticalFilterFromQuery(query), filter);
});

test('filter: an empty filter serializes to an empty query and back', () => {
  assert.equal(tacticalFilterToQuery({}), '');
  assert.deepEqual(tacticalFilterFromQuery(new URLSearchParams('')), {});
  assert.ok(isEmptyTacticalFilter({}));
  assert.ok(!isEmptyTacticalFilter({ side: TACTICAL_SIDES.ct }));
});

test('filter: repeated and comma-separated values both OR', () => {
  const repeated = tacticalFilterFromQuery(new URLSearchParams('buy=eco&buy=force'));
  const commas = tacticalFilterFromQuery(new URLSearchParams('buy=eco,force'));
  assert.deepEqual(repeated, commas);
  assert.deepEqual(repeated.buys, [TACTICAL_BUY_TYPES.eco, TACTICAL_BUY_TYPES.force]);
});

test('filter: an unknown value is dropped and the rest of the filter still applies', () => {
  const parsed = tacticalFilterFromQuery(
    new URLSearchParams('buy=eco&buy=hovercraft&side=sideways&site=b'),
  );
  assert.deepEqual(parsed, { buys: [TACTICAL_BUY_TYPES.eco], sites: [TACTICAL_SITES.b] });
});

test('filter: side and vocabularies are read case-insensitively', () => {
  const parsed = tacticalFilterFromQuery(new URLSearchParams('side=ct&buy=ECO&phase=Overtime'));
  assert.equal(parsed.side, TACTICAL_SIDES.ct);
  assert.deepEqual(parsed.buys, [TACTICAL_BUY_TYPES.eco]);
  assert.equal(parsed.phase, TACTICAL_PHASES.overtime);
});

test('filter: round bounds must be positive integers and correctly ordered', () => {
  assert.deepEqual(tacticalFilterFromQuery(new URLSearchParams('round_from=0&round_to=x')), {});
  assert.deepEqual(tacticalFilterFromQuery(new URLSearchParams('round_from=9&round_to=4')), {});
  assert.deepEqual(tacticalFilterFromQuery(new URLSearchParams('round_from=4&round_to=9')), {
    round_from: 4,
    round_to: 9,
  });
});

test('filter: slots are deduplicated, bounded and sorted', () => {
  const parsed = tacticalFilterFromQuery(new URLSearchParams('slot=7&slot=2&slot=2&slot=99'));
  assert.deepEqual(parsed.slots, [2, 7]);
});

test('filter: the constraint count drives the "N filtros" badge', () => {
  assert.equal(tacticalFilterCount({}), 0);
  assert.equal(
    tacticalFilterCount({ side: TACTICAL_SIDES.ct, buys: [TACTICAL_BUY_TYPES.eco], round_from: 3 }),
    3,
  );
});

test('teamSideInRound: odd halves are the starting side, even halves the swap', () => {
  assert.equal(teamSideInRound(TEAMS, 'home', 1), TACTICAL_SIDES.ct);
  assert.equal(teamSideInRound(TEAMS, 'home', 2), TACTICAL_SIDES.t);
  assert.equal(teamSideInRound(TEAMS, 'away', 1), TACTICAL_SIDES.t);
  assert.equal(teamSideInRound(TEAMS, 'away', 2), TACTICAL_SIDES.ct);
  assert.equal(teamSideInRound(TEAMS, 'ghost', 1), undefined);
});

test('perspective: a team key follows the side swap, a side pins it', () => {
  assert.equal(
    tacticalPerspective(TEAMS, { team_key: 'home' }, { half: 2 }),
    TACTICAL_SIDES.t,
  );
  assert.equal(
    tacticalPerspective(TEAMS, { side: TACTICAL_SIDES.ct }, { half: 2 }),
    TACTICAL_SIDES.ct,
  );
  assert.equal(tacticalPerspective(TEAMS, {}, { half: 1 }), undefined);
});

test('match: a team filter follows the buy across the side swap', () => {
  const first = round({ half: 1, ctBuy: TACTICAL_BUY_TYPES.eco, tBuy: TACTICAL_BUY_TYPES.full });
  const second = round({
    number: 13,
    half: 2,
    ctBuy: TACTICAL_BUY_TYPES.full,
    tBuy: TACTICAL_BUY_TYPES.eco,
  });
  const filter: TacticalFilter = { team_key: 'home', buys: [TACTICAL_BUY_TYPES.eco] };

  assert.ok(roundMatchesFilter(TEAMS, filter, first));
  assert.ok(roundMatchesFilter(TEAMS, filter, second));
});

test('match: the opponent buy is the anti-eco question', () => {
  const antiEco = round({ ctBuy: TACTICAL_BUY_TYPES.full, tBuy: TACTICAL_BUY_TYPES.eco });
  const filter: TacticalFilter = {
    side: TACTICAL_SIDES.ct,
    buys: [TACTICAL_BUY_TYPES.full],
    opponent_buys: [TACTICAL_BUY_TYPES.eco],
  };

  assert.ok(roundMatchesFilter(TEAMS, filter, antiEco));
  assert.ok(!roundMatchesFilter(TEAMS, { ...filter, side: TACTICAL_SIDES.t }, antiEco));
});

test('match: without a perspective a buy matches either side', () => {
  const mixed = round({ ctBuy: TACTICAL_BUY_TYPES.full, tBuy: TACTICAL_BUY_TYPES.eco });
  assert.ok(roundMatchesFilter(TEAMS, { buys: [TACTICAL_BUY_TYPES.eco] }, mixed));
  assert.ok(roundMatchesFilter(TEAMS, { buys: [TACTICAL_BUY_TYPES.full] }, mixed));
  assert.ok(!roundMatchesFilter(TEAMS, { buys: [TACTICAL_BUY_TYPES.pistol] }, mixed));
});

test('match: an outcome without a perspective can never be satisfied', () => {
  const won = round({ winner: TACTICAL_SIDES.ct });
  assert.ok(!roundMatchesFilter(TEAMS, { outcome: TACTICAL_OUTCOMES.win }, won));
  assert.ok(
    roundMatchesFilter(TEAMS, { side: TACTICAL_SIDES.ct, outcome: TACTICAL_OUTCOMES.win }, won),
  );
  assert.ok(
    roundMatchesFilter(TEAMS, { side: TACTICAL_SIDES.t, outcome: TACTICAL_OUTCOMES.loss }, won),
  );
});

test('match: a round with no winner is neither a win nor a loss', () => {
  const unresolved = round({ winner: '' });
  assert.ok(
    !roundMatchesFilter(TEAMS, { side: TACTICAL_SIDES.t, outcome: TACTICAL_OUTCOMES.loss }, unresolved),
  );
});

test('match: tags compose with AND', () => {
  const tagged = round({ tags: ['postplant', 'anti_eco'] });
  assert.ok(roundMatchesFilter(TEAMS, { tags: ['postplant'] }, tagged));
  assert.ok(roundMatchesFilter(TEAMS, { tags: ['postplant', 'anti_eco'] }, tagged));
  assert.ok(!roundMatchesFilter(TEAMS, { tags: ['postplant', 'ace'] }, tagged));
});

test('match: a slot only counts on the perspective side', () => {
  const played = round({
    players: [
      { slot: 0, side: TACTICAL_SIDES.ct },
      { slot: 5, side: TACTICAL_SIDES.t },
    ],
  });
  assert.ok(roundMatchesFilter(TEAMS, { slots: [5] }, played));
  assert.ok(roundMatchesFilter(TEAMS, { side: TACTICAL_SIDES.t, slots: [5] }, played));
  assert.ok(!roundMatchesFilter(TEAMS, { side: TACTICAL_SIDES.ct, slots: [5] }, played));
});

test('match: phase separates regulation from overtime', () => {
  const regulation = round({ number: 4 });
  const overtime = round({ number: 25, overtime: 1 });
  assert.ok(roundMatchesFilter(TEAMS, { phase: TACTICAL_PHASES.regulation }, regulation));
  assert.ok(!roundMatchesFilter(TEAMS, { phase: TACTICAL_PHASES.regulation }, overtime));
  assert.ok(roundMatchesFilter(TEAMS, { phase: TACTICAL_PHASES.overtime }, overtime));
});

test('match: a team that never played the round is filtered out', () => {
  assert.ok(!roundMatchesFilter(TEAMS, { team_key: 'other-map' }, round()));
});

test('filterTacticalRounds: keeps document order', () => {
  const rounds = [round({ number: 1 }), round({ number: 2 }), round({ number: 3 })];
  const kept = filterTacticalRounds(TEAMS, { round_from: 2 }, rounds);
  assert.deepEqual(kept.map((r) => r.number), [2, 3]);
});
