import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalHistogram, TacticalRate } from '../api/tactical.ts';
import type { MetricsReport } from '../api/metrics.ts';
import type { ScoutReport } from '../api/scout.ts';
import { buildReportHtml, escapeHtml, reportFileStem } from './export-html.ts';
import { evidencePath, rateText, scoutSections, secondsText } from './scout-sections.ts';

const MATCH = 'aaaaaaaa-0000-4000-8000-000000000001';

const r = (count: number, total: number): TacticalRate => ({
  count,
  total,
  pct: total > 0 ? Math.round((count / total) * 1000) / 10 : 0,
  reliable: total >= 4,
});
const hist = (median: number, samples: number): TacticalHistogram => ({ bucket_seconds: 5, samples, median, buckets: [] });
const ev = (round: number, tick: number) => ({ match_id: MATCH, round, tick });

function fixture(): ScoutReport {
  return {
    team_id: 'team-1',
    team_name: 'Vitality <B>',
    skipped: [{ match_id: 'bbbbbbbb-0000-4000-8000-000000000002', reason: 'analysis_missing' }],
    filter: {},
    thresholds: {
      min_reliable_sample: 4,
      example_limit: 8,
      early_utility_seconds: 20,
      execute_window_seconds: 20,
      setup_seconds: 15,
      utility_row_limit: 12,
    },
    matches: [
      {
        match_id: MATCH, map: 'de_mirage', played_at: '2026-10-12T18:00:00Z', team_key: 'ct-start',
        opponent: 'MOUZ', score: 13, opponent_score: 9, rounds: 22, wins: 13,
      },
    ],
    maps: [{ map: 'de_mirage', matches: 1, match_wins: 1, rounds: 22, win_rate: r(13, 22), ct_win_rate: r(8, 12), t_win_rate: r(5, 10) }],
    tendencies: {
      filter: {}, round_count: 22, wins: 13, buys: [], matchups: [], sites: [], buy_sites: [], t_patterns: [], ct_patterns: [],
      openings: { rounds: 20, won: r(12, 20), traded_on_loss: r(3, 8), round_win_after_opening_win: r(9, 12), round_win_after_opening_loss: r(4, 8) },
      timings: { first_contact: hist(18, 20), plant: hist(52, 6), round_duration: hist(80, 22) },
      players: [],
    },
    pistols: {
      sides: [{ side: 'T', rounds: 1, share: r(1, 2), win_rate: r(1, 1), examples: [ev(13, 4100)] }],
      rows: [{ side: 'T', pattern: 'fast', site: 'A', rounds: 1, share: r(1, 1), win_rate: r(1, 1), examples: [ev(13, 4100)] }],
    },
    t_defaults: {
      rounds: 6, share: r(6, 6), win_rate: r(3, 6), examples: [ev(15, 9000)],
      patterns: [{ side: 'T', pattern: 'default', site: 'B', rounds: 6, share: r(6, 6), win_rate: r(3, 6), examples: [ev(15, 9000), ev(16, 9900)] }],
      utility: [{ kind: 'smoke', callout: 'TopMid', rounds: 5, share: r(5, 6), count: 5, median_seconds: 7.6, throwers: [{ steamid64: '1', name: 'apEX', count: 5 }], examples: [ev(15, 9100)] }],
      opening_places: [{ place: 'Middle', rounds: 4, share: r(4, 6), won: r(2, 4), examples: [ev(15, 9500)] }],
      first_contact: hist(21.4, 6),
      utility_round_total: 6,
    },
    ct_setups: {
      rounds: 0, share: r(0, 0), win_rate: r(0, 0), examples: [], patterns: [], utility: [], sampled: 0, positions: [],
    },
    executes: [
      {
        site: 'A', rounds: 3, share: r(3, 3), win_rate: r(2, 3), examples: [ev(18, 12000)],
        patterns: [], plants: r(2, 3), plant_time: hist(48, 2), first_contact: hist(30, 3),
        mean_utility: { smoke: 2.3, flash: 1.7, molotov: 1, he: 0 },
        utility: [{ kind: 'molotov', callout: 'BombsiteA', rounds: 3, share: r(3, 3), count: 3, median_seconds: 44, throwers: [], examples: [ev(18, 11800)] }],
      },
    ],
    economy: {
      contexts: [{ context: 'after_pistol_loss', side: 'CT', rounds: 1, share: r(1, 1), win_rate: r(0, 1), examples: [ev(2, 1500)] }],
      rows: [{ context: 'after_pistol_loss', side: 'CT', buy: 'force', rounds: 1, share: r(1, 1), win_rate: r(0, 1), examples: [ev(2, 1500)] }],
    },
    awp: {
      sides: [{ side: 'CT', rounds: 12, kill_rounds: r(5, 12), kills: 7, opening_kills: 3 }],
      rows: [{ side: 'CT', place: 'Jungle', kills: 4, share: r(4, 7), opening_kills: 2, players: [{ steamid64: '2', name: 'ZywOo', count: 4 }], examples: [ev(4, 2200)] }],
    },
    openings: [
      {
        steamid64: '3', name: 'flameZ', side: 'T', rounds: 10, attempts: r(4, 10), won: r(3, 4), traded_on_loss: r(1, 1),
        places: [{ place: 'Palace', count: 3 }], examples: [ev(14, 5000)],
      },
    ],
  };
}

test('a rate always carries its counts and an empty one says so', () => {
  assert.equal(rateText(r(10, 23)), '43 % (10/23)');
  assert.equal(rateText(r(0, 0)), 'Sin datos');
  assert.equal(secondsText(42.3), '0:42');
  assert.equal(secondsText(75), '1:15');
  assert.equal(secondsText(Number.NaN), '');
});

test('every row that counts rounds links its examples to the round and tick', () => {
  assert.equal(evidencePath(ev(13, 4100)), `/partidas/${MATCH}?tab=tactica&round=13&tick=4100`);
  const sections = scoutSections(fixture());
  assert.deepEqual(
    sections.map((section) => section.id),
    ['resumen', 'pistolas', 'defaults-t', 'setups-ct', 'ejecuciones', 'economia', 'awp', 'aperturas'],
  );
  for (const section of sections) {
    for (const table of section.tables) {
      for (const row of table.rows) {
        assert.equal(row.length, table.columns.length, `${table.caption}: a row must fill every column`);
      }
      if (!table.columns.includes('Rondas de ejemplo')) continue;
      for (const row of table.rows) {
        const cell = row[table.columns.indexOf('Rondas de ejemplo')];
        assert.ok(cell?.evidence !== undefined, `${table.caption}: the example column must hold evidence`);
      }
    }
  }
});

test('a small sample is marked instead of hidden', () => {
  const pistols = scoutSections(fixture()).find((section) => section.id === 'pistolas');
  const winRate = pistols?.tables[0]?.rows[0]?.[5];
  assert.equal(winRate?.text, '100 % (1/1)');
  assert.equal(winRate?.lowSample, true);
});

test('the exported HTML is self-contained, escaped and links every example', () => {
  const html = buildReportHtml({
    title: 'Vitality <script>alert(1)</script>',
    teamName: 'Vitality <B>',
    side: 'T',
    notes: 'Fuerzan tras perder la pistola & salen por B.',
    scout: fixture(),
    origin: 'http://127.0.0.1:3000',
    generatedAt: new Date('2026-10-30T10:00:00Z'),
  });
  assert.ok(html.startsWith('<!doctype html>'));
  assert.equal(html.includes('<script'), false);
  assert.ok(html.includes('Vitality &lt;script&gt;alert(1)&lt;/script&gt;'));
  assert.ok(html.includes('Fuerzan tras perder la pistola &amp; salen por B.'));
  assert.ok(html.includes(`href="http://127.0.0.1:3000/partidas/${MATCH}?tab=tactica&amp;round=15&amp;tick=9100"`));
  assert.equal(/(src|href)="(?!http:\/\/127\.0\.0\.1:3000\/)/.test(html), false, 'no resource outside the exporting Studio');
  assert.equal(/[–—]/.test(html), false, 'no en or em dashes in the exported text');
  assert.ok(html.includes('La partida aún no tiene análisis táctico.'));

  const offline = buildReportHtml({
    title: 'x', teamName: 'x', side: '', notes: '', scout: fixture(), origin: '', generatedAt: new Date('2026-10-30T10:00:00Z'),
  });
  assert.equal(offline.includes('href='), false);
  assert.ok(offline.includes('Mirage R15'));
});

test('the exported HTML carries the advanced metrics when the report has them', () => {
  const metrics: MetricsReport = {
    skipped: [],
    model: {
      version: 'wp1', alive_coef: 0.9, equip_coef: 1, clock_coef: 1.2, plant_coef: -1.1, bomb_coef: -1.5,
      round_seconds: 115, bomb_seconds: 40, min_equip: 200, map_priors: {},
    },
    thresholds: { duel_even_ratio: 1.5, flash_min_seconds: 1, trade_seconds: 5, setup_seconds: 15, example_limit: 8 },
    matches: 1, rounds: 22, utility_matches: 1, players: [], roles: [],
  };
  const base = { title: 'x', teamName: 'x', side: '' as const, notes: '', scout: fixture(), origin: '', generatedAt: new Date('2026-10-30T10:00:00Z') };
  const html = buildReportHtml({ ...base, metrics });
  assert.ok(html.includes('<section id="metricas-impacto">'));
  assert.ok(html.includes('Probabilidad de victoria wp1'));
  assert.equal(/[–—]/.test(html), false);
  assert.equal(buildReportHtml(base).includes('metricas-impacto'), false);
});

test('download names are plain ASCII slugs', () => {
  assert.equal(reportFileStem('Vitality: Mirage y Nuke (final)'), 'informe-vitality-mirage-y-nuke-final');
  assert.equal(reportFileStem('Señal España'), 'informe-senal-espana');
  assert.equal(reportFileStem('***'), 'informe-scouting');
  assert.equal(escapeHtml(`"'<>&`), '&quot;&#39;&lt;&gt;&amp;');
});
