import assert from 'node:assert/strict';
import test from 'node:test';
import { studioJobsRunning, tasklistHasStudioJob } from './studio-jobs.ts';

const IDLE = '"System Idle Process","0","Services","0","8 K"\r\n"zv-orchestrator.exe","4120","Console","1","40.120 K"\r\n"cs2.exe","9000","Console","1","2.000.000 K"\r\n';

test('a capture or a render is a Studio job; the orchestrator and the game alone are not', () => {
  assert.equal(tasklistHasStudioJob(IDLE), false);
  assert.equal(tasklistHasStudioJob(`${IDLE}"zv-recorder.exe","5000","Console","1","30.000 K"\r\n`), true);
  assert.equal(tasklistHasStudioJob(`${IDLE}"ZV-EDITOR.EXE","5001","Console","1","30.000 K"\r\n`), true);
  assert.equal(tasklistHasStudioJob('"my-zv-editor.exe.bak","1","Console","1","1 K"\r\n'), false);
});

test('when the process list cannot be read, Studio counts as busy so an update never cuts a job', async () => {
  assert.equal(await studioJobsRunning(async () => IDLE), false);
  assert.equal(await studioJobsRunning(async () => `${IDLE}"zv-editor.exe","1","Console","1","1 K"`), true);
  assert.equal(
    await studioJobsRunning(async () => {
      throw new Error('tasklist failed');
    }),
    true,
  );
});
