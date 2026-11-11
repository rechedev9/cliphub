import test from 'node:test';
import assert from 'node:assert/strict';
import type { PlaybookTactic } from '../api/playbook.ts';
import { EDITOR_PANELS, chosenZones, keptText, panelConditions, panelRows } from './tactic-editor.ts';
import { TACTIC_KINDS } from '../api/playbook.ts';

function rate(count: number, total: number) {
  return { count, total, pct: total === 0 ? 0 : (count / total) * 100, reliable: total >= 5 };
}

function tactic(key: string, kind: PlaybookTactic['kind'], rounds: number): PlaybookTactic {
  return { key, kind, value: key, rounds, share: rate(rounds, 10), win_rate: rate(rounds, rounds), refs: [] };
}

const FULL = [tactic('category:rush', 'category', 6), tactic('buy:full', 'buy', 8), tactic('zone:Mid', 'zone', 4), tactic('utility:smoke:Mid', 'utility', 3)];

test('tactic editor: every tactic kind belongs to exactly one panel', () => {
  const kinds = EDITOR_PANELS.flatMap((panel) => panel.kinds);
  assert.deepEqual([...kinds].sort(), [...TACTIC_KINDS].sort());
});

test('tactic editor: a panel lists the kept tactics of its kinds, grouped by kind in panel order', () => {
  const view = { tactics: FULL, combo: { with: [], without: [] } };
  const rows = panelRows({ playbook: { tactics: FULL }, view, kinds: ['opening', 'zone', 'utility'] });
  assert.deepEqual(rows.map((row) => [row.kind, row.tactics.map((entry) => entry.key)]), [
    ['zone', ['zone:Mid']],
    ['utility', ['utility:smoke:Mid']],
  ]);
});

test('tactic editor: a condition no kept round shows stays as a zero row so it can be taken off', () => {
  const view = { tactics: [FULL[0] ?? tactic('x', 'site', 0)], combo: { with: ['category:rush'], without: ['zone:Mid'] } };
  const rows = panelRows({ playbook: { tactics: FULL }, view, kinds: ['zone'] });
  assert.equal(rows.length, 1);
  assert.equal(rows[0]?.tactics[0]?.key, 'zone:Mid');
  assert.equal(rows[0]?.tactics[0]?.rounds, 0);
});

test('tactic editor: panels count their own conditions, and zones read back from the keys', () => {
  const combo = { with: ['category:rush', 'zone:Mid'], without: ['buy:full'] };
  assert.equal(panelConditions({ combo, playbook: { tactics: FULL }, kinds: ['category', 'site'] }), 1);
  assert.equal(panelConditions({ combo, playbook: { tactics: FULL }, kinds: ['buy', 'opponent_buy'] }), 1);
  assert.equal(panelConditions({ combo, playbook: { tactics: FULL }, kinds: ['utility'] }), 0);
  assert.deepEqual(chosenZones(combo), ['Mid']);
  assert.equal(keptText({ kept: 1, total: 1 }), '1 de 1 ronda');
  assert.equal(keptText({ kept: 4, total: 30 }), '4 de 30 rondas');
});
