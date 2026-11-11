import test from 'node:test';
import assert from 'node:assert/strict';
import type { DetectedPlay, LibraryPlay } from '../api/stratbook.ts';
import {
  importSummary,
  parseStratbookFile,
  playFromDetected,
  playKinds,
  playShapeLabel,
  shareLabel,
  stratbookFileName,
} from './plays.ts';

const MATCH = '11111111-1111-4111-8111-111111111111';

const draft: LibraryPlay = {
  id: '00000000-0000-0000-0000-000000000000',
  title: '',
  team_id: '22222222-2222-4222-8222-222222222222',
  team_name: 'Vitality',
  map: 'de_mirage',
  side: 'T',
  kind: 'execute',
  site: 'A',
  origin: 'round',
  positions_seconds: 15,
  grenades: [{ kind: 'smoke', seconds: 12.5, pos: [1, 2, 3], callout: 'CT', lineup_id: 'mirage-ct' }],
  positions: [{ player: 'apEX', steamid64: '1', pos: [4, 5, 6] }],
  notes: '',
  examples: [{ match_id: MATCH, round: 7, tick: 9000 }],
  status: 'active',
  tags: [],
  roles: [],
  created_at: '',
  updated_at: '',
};

test('a detected play becomes a play with every round as an example, capped at 50', () => {
  const detected: DetectedPlay = {
    map: 'de_nuke', side: 'CT', kind: 'hold', site: 'B', rounds: 60,
    share: { count: 60, total: 80, pct: 75, reliable: true },
    win_rate: { count: 30, total: 60, pct: 50, reliable: true },
    grenades: [], positions: [],
    seed: { match_id: MATCH, round: 1, tick: 1 },
    examples: Array.from({ length: 60 }, (_, i) => ({ match_id: MATCH, round: i + 1, tick: i })),
  };
  const input = playFromDetected(detected, { teamId: 't', title: 'Setup B', positionSeconds: 15 });
  assert.equal(input.origin, 'detected');
  assert.equal(input.examples.length, 50);
  // Without positions the play does not claim a positions second.
  assert.equal(input.positions_seconds, 0);
});

test('play labels follow the round classifier vocabulary', () => {
  assert.equal(playShapeLabel({ kind: 'execute', site: 'A' }), 'Ejecución A');
  assert.equal(playShapeLabel({ kind: 'hold', site: 'none' }), 'Defensa');
  assert.ok(playKinds('T').includes('execute'));
  assert.equal(playKinds('CT').includes('execute'), false);
  assert.equal(shareLabel(0.666), '67 %');
  assert.equal(shareLabel(undefined), '');
});

test('a stratbook file is recognised by its format and version', () => {
  const file = { format: 'cliphub-stratbook', version: 1, exported_at: '2026-10-30T10:00:00.000Z', plays: [draft] };
  const parsed = parseStratbookFile(JSON.stringify(file));
  assert.ok(parsed.ok);
  assert.equal(parsed.file.plays.length, 1);
  assert.equal(parseStratbookFile('nope').ok, false);
  assert.equal(parseStratbookFile(JSON.stringify({ format: 'other', version: 1, plays: [] })).ok, false);
  assert.ok(parseStratbookFile(JSON.stringify({ ...file, version: 2 })).ok);
  const newer = parseStratbookFile(JSON.stringify({ ...file, version: 3 }));
  assert.equal(newer.ok, false);
  assert.match(newer.ok ? '' : newer.error, /versión 3/);
});

test('the export file name carries the team and the day', () => {
  const day = new Date('2026-10-30T22:00:00.000Z');
  assert.equal(stratbookFileName(day, 'Team Vitality á'), 'libreta-team-vitality-a-2026-10-30.json');
  assert.equal(stratbookFileName(day), 'libreta-2026-10-30.json');
});

test('a whole Libreta file keeps its other sections for the import, and drops empty or broken ones', () => {
  const file = {
    format: 'cliphub-stratbook', version: 2, exported_at: '2026-11-07T10:00:00.000Z', plays: [],
    lineups: [{ id: 'l' }], calls: [{ id: 'c' }], callout_names: [], team_roles: 'nope', extra: [1],
  };
  const parsed = parseStratbookFile(JSON.stringify(file));
  assert.ok(parsed.ok);
  assert.deepEqual(Object.keys(parsed.file).sort(), ['calls', 'exported_at', 'format', 'lineups', 'plays', 'version']);
});

test('the import summary says what the plays did, one sentence per kind of result', () => {
  const plays = { imported: 3, duplicates: 1, rejected: [], plays: [] };
  assert.equal(importSummary(plays), 'Importadas 3 jugadas. 1 ya estaba en la Libreta.');
  assert.equal(
    importSummary({ ...plays, imported: 1, duplicates: 0, rejected: [{ index: 4, title: '', error: 'x' }] }),
    'Importada 1 jugada. 1 no se pudo leer: n.º 5.',
  );
  assert.equal(importSummary({ ...plays, imported: 0, duplicates: 0 }), 'Importadas 0 jugadas.');
});

test('the import summary adds every section the file held, in tab order, and skips plays it did not hold', () => {
  const result = {
    imported: 2, duplicates: 0, rejected: [], plays: [],
    sections: {
      callout_names: { imported: 14, duplicates: 0, rejected: 0 },
      calls: { imported: 1, duplicates: 2, rejected: 0 },
      lineups: { imported: 0, duplicates: 3, rejected: 1 },
      team_roles: { imported: 2, duplicates: 0, rejected: 0 },
    },
  };
  assert.equal(
    importSummary(result),
    'Importadas 2 jugadas. Lineups: 3 ya estaban en la Libreta, 1 no se pudo leer. Hojas de roles: 2 importadas. '
      + 'Llamadas: 1 importada, 2 ya estaban en la Libreta. Nombres de callouts: 14 importados.',
  );
  assert.equal(
    importSummary({ ...result, imported: 0, sections: { calls: { imported: 5, duplicates: 0, rejected: 0 } } }),
    'Llamadas: 5 importadas.',
  );
});
