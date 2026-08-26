import assert from 'node:assert/strict';
import test from 'node:test';
import { ZERO_STATS } from './api/jobs-index.ts';
import {
  DEMO_CHEATER_SINGLE_FILE_HINT,
  matchFromScan,
  pickCheaterDetectDemo,
} from './cheater-detect-ingest.ts';

test('pickCheaterDetectDemo admits one file and rejects empty or series drops', () => {
  const one = new File(['x'], 'match.dem');
  const two = new File(['y'], 'map2.dem');
  const cases: Array<{ name: string; files: File[]; want: ReturnType<typeof pickCheaterDetectDemo> }> = [
    { name: 'empty', files: [], want: { ok: false, error: DEMO_CHEATER_SINGLE_FILE_HINT } },
    { name: 'two demos', files: [one, two], want: { ok: false, error: DEMO_CHEATER_SINGLE_FILE_HINT } },
    { name: 'one demo', files: [one], want: { ok: true, file: one } },
  ];
  for (const row of cases) {
    assert.deepEqual(pickCheaterDetectDemo(row.files), row.want, row.name);
  }
});

test('matchFromScan uses the roster map and keeps a filename fallback', () => {
  const createdAt = '2026-08-14T18:15:37.000Z';
  const cases: Array<{
    name: string;
    roster?: { map: string; scoreCt: number; scoreT: number; rounds: number };
    wantMap: string;
  }> = [
    { name: 'inferno', roster: { map: 'de_inferno', scoreCt: 13, scoreT: 9, rounds: 22 }, wantMap: 'Inferno' },
    { name: 'no roster', wantMap: 'match730.dem' },
  ];
  for (const row of cases) {
    const match = matchFromScan({
      jobId: 'job-1',
      fileName: 'match730.dem',
      createdAt,
      roster: row.roster,
    });
    assert.equal(match.id, 'job-1', row.name);
    assert.equal(match.map, row.wantMap, row.name);
    assert.equal(match.source, 'upload', row.name);
    assert.equal(match.playedAt, createdAt, row.name);
    assert.deepEqual(match.stats, ZERO_STATS, row.name);
  }
});
