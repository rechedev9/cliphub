import assert from 'node:assert/strict';
import test from 'node:test';
import { requiredTests, verifyEvidence } from './ci-backend-evidence.mjs';

function event(key, Action) {
  const split = key.lastIndexOf('/');
  return JSON.stringify({ Package: key.slice(0, split), Test: key.slice(split + 1), Action });
}
const passes = requiredTests.map(key => event(key, 'pass'));

test('requires all critical flow passes', async () => {
  await verifyEvidence(passes);
  await assert.rejects(verifyEvidence([]), /Missing critical flow/);
  await assert.rejects(verifyEvidence(passes.slice(1)), /Missing critical flow/);
});

test('rejects skipped canaries and skipped scenarios even when the parent passes', async () => {
  await assert.rejects(verifyEvidence([event(requiredTests[0], 'skip'), ...passes]), /skipped/);
  const subtest = JSON.stringify({ Package: 'github.com/rechedev9/cliphub/internal/recording', Test: 'TestGeneratedHLAEScriptRunsInMIRVSimulator/scenario', Action: 'skip' });
  await assert.rejects(verifyEvidence([subtest, ...passes]), /skipped/);
});

test('allows unrelated optional fixtures but rejects failures and invalid logs', async () => {
  await verifyEvidence([event('optional/fixture', 'skip'), ...passes]);
  await assert.rejects(verifyEvidence([...passes, event('other/test', 'fail')]), /failed/);
  await assert.rejects(verifyEvidence(['truncated JSON']), SyntaxError);
});
