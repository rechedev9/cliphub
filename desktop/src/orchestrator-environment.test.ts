import test from 'node:test';
import assert from 'node:assert/strict';
import { createOrchestratorEnvironment } from './orchestrator-environment.ts';

test('pins the bundled recorder over stale runtime tool overrides', () => {
  const cases = [
    {
      name: 'missing override',
      toolEnvironment: {},
    },
    {
      name: 'obsolete developer checkout',
      toolEnvironment: {
        ZV_RECORDER_PATH: String.raw`C:\Users\player\Documents\fragforge\bin\zv-recorder.exe`,
      },
    },
  ];

  for (const tc of cases) {
    const bundledRecorder = String.raw`C:\Users\player\AppData\Local\Programs\ClipHub Studio\resources\bin\zv-recorder.exe`;
    const environment = createOrchestratorEnvironment({
      dataDir: String.raw`C:\Users\player\AppData\Roaming\cliphub-studio\data`,
      httpAddress: '127.0.0.1:23947',
      musicDir: String.raw`C:\Users\player\AppData\Roaming\cliphub-studio\data\music`,
      recorderPath: bundledRecorder,
      securityEnvironment: { ZV_DISCOVERY_SECRET: 'discovery' },
      toolEnvironment: tc.toolEnvironment,
    });

    assert.equal(environment.ZV_RECORDER_PATH, bundledRecorder, tc.name);
  }
});

test('preserves the remaining orchestrator runtime environment', () => {
  const environment = createOrchestratorEnvironment({
    dataDir: 'data',
    httpAddress: '127.0.0.1:45975',
    musicDir: 'music',
    recorderPath: 'bin/zv-recorder',
    securityEnvironment: { ZV_DISCOVERY_SECRET: 'discovery' },
    toolEnvironment: { ZV_HLAE_PATH: 'tools/HLAE.exe' },
  });

  assert.deepEqual(environment, {
    ZV_DATABASE_URL: 'sqlite',
    ZV_DATA_DIR: 'data',
    ZV_DISCOVERY_SECRET: 'discovery',
    ZV_HLAE_PATH: 'tools/HLAE.exe',
    ZV_HTTP_ADDR: '127.0.0.1:45975',
    ZV_MUSIC_DIR: 'music',
    ZV_RECORDER_PATH: 'bin/zv-recorder',
  });
});
