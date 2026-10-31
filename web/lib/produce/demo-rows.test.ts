import test from 'node:test';
import assert from 'node:assert/strict';
import type { Match, Video } from '../api/types.ts';
import { HUB_ROW_STAGE, toOutput, type HubMatch, type HubRowStage } from '../clips/hub.ts';
import { demoDeletePrompt, oneRowPerDemo } from './demo-rows.ts';

function row(id: string, stage: HubRowStage, demoSha256?: string, shortAt?: number): HubMatch {
  const match: Match = {
    id, map: 'Mirage', score: '', playedAt: '2026-10-30T20:00:00Z', stats: { kills: 0, deaths: 0, assists: 0, mvps: 0, kd: 0 },
    decentPlays: 0, ...(demoSha256 === undefined ? {} : { demoSha256 }),
  };
  const video: Video = { id: `v-${id}`, jobId: id, title: id, map: 'de_mirage', score: '', mode: 'clean', status: 'ready', createdAt: shortAt ?? 0 };
  return { match, stage, shorts: shortAt === undefined ? [] : [toOutput(video)], fulls: [] };
}

test('jobs of the same demo fold into one row that opens the newest ready job', () => {
  // 5kMirage after a Short and two Grabar POV jobs from Analizar, newest first.
  const rows = [
    row('pov-2', HUB_ROW_STAGE.parsing, 'sha-mirage'),
    row('pov-1', HUB_ROW_STAGE.ready, 'sha-mirage'),
    row('anubis', HUB_ROW_STAGE.ready, 'sha-anubis'),
    row('short', HUB_ROW_STAGE.ready, 'sha-mirage', 5),
  ];
  const groups = oneRowPerDemo(rows);
  assert.deepEqual(groups.map((group) => [group.match.id, group.jobIds, group.shorts.map((output) => output.id)]), [
    ['pov-1', ['pov-2', 'pov-1', 'short'], ['v-short']],
    ['anubis', ['anubis'], []],
  ]);
});

test('a demo with no ready job shows its newest job, and jobs without a checksum stay apart', () => {
  const groups = oneRowPerDemo([row('b', HUB_ROW_STAGE.failed, 'sha'), row('a', HUB_ROW_STAGE.parsing, 'sha'), row('x', HUB_ROW_STAGE.ready), row('y', HUB_ROW_STAGE.ready)]);
  assert.deepEqual(groups.map((group) => group.match.id), ['b', 'x', 'y']);
});

test('the delete prompt of a folded demo counts the videos of all its jobs', () => {
  const [mirage, anubis] = oneRowPerDemo([row('pov', HUB_ROW_STAGE.ready, 'sha-mirage', 2), row('short', HUB_ROW_STAGE.ready, 'sha-mirage', 1), row('anubis', HUB_ROW_STAGE.ready, 'sha-anubis')]);
  assert.ok(mirage !== undefined && anubis !== undefined);
  assert.equal(demoDeletePrompt(mirage), '¿Borrar con 2 vídeos?');
  assert.equal(demoDeletePrompt(anubis), '¿Borrar?');
});
