import test from 'node:test';
import assert from 'node:assert/strict';
import type { DetectedPlay, LibraryPlay } from '../api/stratbook.ts';
import {
  parseStratbookFile,
  playFromDetected,
  playFromDraft,
  playKinds,
  playShapeLabel,
  shareLabel,
  stratbookFileName,
  suggestPlayTitle,
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
  created_at: '',
  updated_at: '',
};

test('a play from a round keeps its round as the example and trims the coach text', () => {
  const input = playFromDraft(draft, '  Ejecución A  ', ' humo CT primero ');
  assert.equal(input.title, 'Ejecución A');
  assert.equal(input.notes, 'humo CT primero');
  assert.equal(input.origin, 'round');
  assert.deepEqual(input.examples, draft.examples);
  assert.equal('id' in input, false);
});

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
  assert.equal(suggestPlayTitle({ kind: 'hold', site: 'B', side: 'CT' }, 4), 'Defensa B (CT, ronda 4)');
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
  const newer = parseStratbookFile(JSON.stringify({ ...file, version: 2 }));
  assert.equal(newer.ok, false);
  assert.match(newer.ok ? '' : newer.error, /versión 2/);
});

test('the export file name carries the team and the day', () => {
  const day = new Date('2026-10-30T22:00:00.000Z');
  assert.equal(stratbookFileName(day, 'Team Vitality á'), 'stratbook-team-vitality-a-2026-10-30.json');
  assert.equal(stratbookFileName(day), 'stratbook-2026-10-30.json');
});
