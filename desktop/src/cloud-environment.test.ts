import test from 'node:test';
import assert from 'node:assert/strict';
import { cloudEnvironment } from './cloud-environment.ts';

const CASES: Array<{ name: string; source: NodeJS.ProcessEnv; want: NodeJS.ProcessEnv }> = [
  { name: 'no override keeps the orchestrator default portal', source: {}, want: { ZV_STUDIO_VERSION: '5.4.4' } },
  {
    name: 'a portal override is forwarded',
    source: { ZV_CLOUD_URL: 'http://127.0.0.1:3000' },
    want: { ZV_STUDIO_VERSION: '5.4.4', ZV_CLOUD_URL: 'http://127.0.0.1:3000' },
  },
  {
    name: 'off reaches the orchestrator so it can disable the client',
    source: { ZV_CLOUD_URL: ' off ' },
    want: { ZV_STUDIO_VERSION: '5.4.4', ZV_CLOUD_URL: 'off' },
  },
  { name: 'whitespace counts as unset', source: { ZV_CLOUD_URL: '   ' }, want: { ZV_STUDIO_VERSION: '5.4.4' } },
  {
    name: 'a version in the launch environment never replaces the real one',
    source: { ZV_STUDIO_VERSION: '0.0.1', ZV_BRIDGE_TOKEN: 'nope' },
    want: { ZV_STUDIO_VERSION: '5.4.4' },
  },
];

for (const testCase of CASES) {
  test(`cloudEnvironment: ${testCase.name}`, () => {
    assert.deepEqual(cloudEnvironment(testCase.source, '5.4.4'), testCase.want);
  });
}
