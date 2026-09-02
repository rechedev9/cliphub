import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import { FULL_DEMO_EDIT, fullDemoEdit } from '../full-demo.ts';
import { reelContractMatches, reelIdentity, shouldReuseReelIntent } from './reel-identity.ts';
import { OVERLAY_THEME } from './types.ts';

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
    {
      name: '9:16 plus recap flags stays a shorts identity',
      input: {
        matchId: JOB,
        playIds: PLAYS,
        editConfig: { ...DEFAULT_EDIT_CONFIG, matchRecap: true, voiceComms: true, nativeHud: true },
      },
      want: `${JOB}__seg-001_seg-002`,
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

test('in-flight Full Demo reuses the reel when overlay theme changes', () => {
  const orange = intent({
    videoId: `${JOB}__full-demo`,
    segmentIds: [],
    variant: 'gameplay-pov-60',
    editConfig: fullDemoEdit(OVERLAY_THEME.faceitOrange),
  });
  const violetInput = {
    matchId: JOB,
    playIds: [] as string[],
    mode: 'clean' as const,
    variant: 'gameplay-pov-60',
    editConfig: fullDemoEdit(OVERLAY_THEME.neonViolet),
  };
  assert.equal(reelContractMatches(orange, violetInput), false);
  assert.equal(shouldReuseReelIntent({ status: 'recording' }, orange, violetInput), true);
  assert.equal(shouldReuseReelIntent({ status: 'queued' }, orange, violetInput), true);
  assert.equal(shouldReuseReelIntent({ status: 'composing' }, orange, violetInput), true);
  assert.equal(shouldReuseReelIntent({ status: 'ready' }, orange, violetInput), false);
  assert.equal(shouldReuseReelIntent({ status: 'failed' }, orange, violetInput), false);
  assert.equal(
    shouldReuseReelIntent({ status: 'recording' }, orange, {
      matchId: JOB,
      playIds: PLAYS,
      mode: 'clean',
      variant: 'viral-60-clean',
      editConfig: DEFAULT_EDIT_CONFIG,
    }),
    false,
  );
});
