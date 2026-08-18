import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import { reelContractMatches, reelIdentity } from './reel-identity.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const PLAYS = ['seg-001', 'seg-002'];

function intent(over: Partial<ReelIntent> = {}): ReelIntent {
  return {
    videoId: `${JOB}__${PLAYS.join('_')}`,
    jobId: JOB,
    segmentIds: PLAYS,
    mode: 'clean',
    variant: 'viral-60-clean',
    editConfig: { ...DEFAULT_EDIT_CONFIG },
    title: 't',
    map: 'Inferno',
    score: '',
    createdAt: 1,
    ...over,
  };
}

test('reelIdentity uses a distinct Full Demo slot', () => {
  const cases: Array<{ name: string; input: Parameters<typeof reelIdentity>[0]; want: string }> = [
    { name: 'shorts all plays', input: { matchId: JOB, playIds: PLAYS }, want: `${JOB}__seg-001_seg-002` },
    {
      name: 'full demo ignores play ids',
      input: { matchId: JOB, playIds: PLAYS, editConfig: FULL_DEMO_EDIT },
      want: `${JOB}__full-demo`,
    },
  ];
  for (const tc of cases) {
    assert.equal(reelIdentity(tc.input), tc.want, tc.name);
  }
});

test('reelContractMatches refuses a Shorts reel as Full Demo', () => {
  const shorts = intent();
  const cases: Array<{ name: string; existing: ReelIntent; input: Parameters<typeof reelContractMatches>[1]; want: boolean }> = [
    {
      name: 'same shorts contract reuses',
      existing: shorts,
      input: { matchId: JOB, playIds: PLAYS, mode: 'clean', variant: 'viral-60-clean', editConfig: DEFAULT_EDIT_CONFIG },
      want: true,
    },
    {
      name: 'same plays landscape is a new reel',
      existing: shorts,
      input: {
        matchId: JOB,
        playIds: PLAYS,
        mode: 'clean',
        variant: 'viral-60-clean',
        editConfig: { ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9' },
      },
      want: false,
    },
    {
      name: 'same plays with music is a new reel',
      existing: shorts,
      input: { matchId: JOB, playIds: PLAYS, mode: 'music', songId: 'phonk-01', variant: 'viral-60-clean', editConfig: DEFAULT_EDIT_CONFIG },
      want: false,
    },
    {
      name: 'full demo does not reuse shorts',
      existing: shorts,
      input: { matchId: JOB, playIds: PLAYS, mode: 'clean', variant: 'viral-60-clean', editConfig: FULL_DEMO_EDIT },
      want: false,
    },
  ];
  for (const tc of cases) {
    assert.equal(reelContractMatches(tc.existing, tc.input), tc.want, tc.name);
  }
});
