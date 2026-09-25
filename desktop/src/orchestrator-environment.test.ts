import test from 'node:test';
import assert from 'node:assert/strict';
import { bridgeEnvironment } from './bridge-environment.ts';
import { createOrchestratorEnvironment } from './orchestrator-environment.ts';
import { steamEnvironment } from './steam-environment.ts';

test('pins the bundled overlay renderer and supplies the dev entry only when needed', () => {
  const environment = createOrchestratorEnvironment({ dataDir: 'data', httpAddress: '127.0.0.1:8090', musicDir: 'music', recorderPath: 'recorder.exe', securityEnvironment: {}, toolEnvironment: { ZV_OVERLAY_RENDERER_PATH: 'stale.exe' }, overlayRendererPath: 'Studio.exe', overlayRendererApp: 'desktop' });
  assert.equal(environment.ZV_OVERLAY_RENDERER_PATH, 'Studio.exe');
  assert.equal(environment.ZV_OVERLAY_RENDERER_APP, 'desktop');

  const packaged = createOrchestratorEnvironment({ dataDir: 'data', httpAddress: '127.0.0.1:8090', musicDir: 'music', recorderPath: 'recorder.exe', securityEnvironment: {}, toolEnvironment: {}, overlayRendererPath: 'Studio.exe' });
  assert.equal(packaged.ZV_OVERLAY_RENDERER_PATH, 'Studio.exe');
  assert.equal('ZV_OVERLAY_RENDERER_APP' in packaged, false);
});

test('carries Steam credentials and bridge settings without displacing the recorder pin', () => {
  const environment = createOrchestratorEnvironment({
    dataDir: 'data',
    httpAddress: '127.0.0.1:8080',
    musicDir: 'music',
    recorderPath: 'bin/zv-recorder.exe',
    securityEnvironment: {},
    toolEnvironment: { ZV_HLAE_PATH: 'tools/HLAE.exe', ZV_RECORDER_PATH: 'stale/zv-recorder.exe' },
    steamEnvironment: steamEnvironment({ ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: 'pw' }),
    bridgeEnvironment: bridgeEnvironment({
      ZV_BRIDGE_URL: 'https://portal.example',
      ZV_BRIDGE_TOKEN: 'abc',
    }),
  });

  assert.deepEqual(environment, {
    ZV_DATABASE_URL: 'sqlite',
    ZV_DATA_DIR: 'data',
    GOLANG_PROTOBUF_REGISTRATION_CONFLICT: 'ignore',
    ZV_BRIDGE_TOKEN: 'abc',
    ZV_BRIDGE_URL: 'https://portal.example',
    ZV_HLAE_PATH: 'tools/HLAE.exe',
    ZV_HTTP_ADDR: '127.0.0.1:8080',
    ZV_MUSIC_DIR: 'music',
    ZV_RECORDER_PATH: 'bin/zv-recorder.exe',
    ZV_STEAM_PASSWORD: 'pw',
    ZV_STEAM_USERNAME: 'user',
  });
});

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
    GOLANG_PROTOBUF_REGISTRATION_CONFLICT: 'ignore',
    ZV_DISCOVERY_SECRET: 'discovery',
    ZV_HLAE_PATH: 'tools/HLAE.exe',
    ZV_HTTP_ADDR: '127.0.0.1:45975',
    ZV_MUSIC_DIR: 'music',
    ZV_RECORDER_PATH: 'bin/zv-recorder',
  });
});
