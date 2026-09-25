import test from 'node:test';
import assert from 'node:assert/strict';
import { steamEnvironment } from './steam-environment.ts';

const CASES: Array<{ name: string; source: NodeJS.ProcessEnv; want: NodeJS.ProcessEnv }> = [
  { name: 'nothing set', source: {}, want: {} },
  {
    name: 'all three set',
    source: { ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: 'pw', ZV_STEAM_GUARD: 'secret' },
    want: { ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: 'pw', ZV_STEAM_GUARD: 'secret' },
  },
  {
    name: 'empty and whitespace values are dropped, not forwarded as configured',
    source: { ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: '', ZV_STEAM_GUARD: '   ' },
    want: { ZV_STEAM_USERNAME: 'user' },
  },
  {
    name: 'unrelated variables are never forwarded',
    source: { ZV_STEAM_USERNAME: 'user', PATH: '/bin', ZV_HLAE_PATH: 'C:\\HLAE.exe' },
    want: { ZV_STEAM_USERNAME: 'user' },
  },
];

for (const testCase of CASES) {
  test(`steamEnvironment: ${testCase.name}`, () => {
    assert.deepEqual(steamEnvironment(testCase.source), testCase.want);
  });
}
