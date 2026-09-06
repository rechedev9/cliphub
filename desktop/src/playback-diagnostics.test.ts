import assert from 'node:assert/strict';
import test from 'node:test';
import { createPlaybackDiagnostics, readPlaybackInfo } from './playback-diagnostics.ts';

test('withholds playback diagnostics until Electron reports GPU information', () => {
  assert.deepEqual(createPlaybackDiagnostics({
    gpuInformationReady: false,
    electronVersion: '43.0.0',
    chromiumVersion: '150.0.0.0',
    hardwareAccelerationEnabled: true,
    videoDecodeStatus: 'enabled',
  }), { available: false, state: 'initializing' });
});

test('returns only the bounded global playback capability summary', () => {
  assert.deepEqual(createPlaybackDiagnostics({
    gpuInformationReady: true,
    electronVersion: '43.0.0',
    chromiumVersion: '150.0.7339.2',
    hardwareAccelerationEnabled: true,
    videoDecodeStatus: 'enabled_on',
  }), {
    available: true,
    state: 'ready',
    electronVersion: '43.0.0',
    chromiumVersion: '150.0.7339.2',
    hardwareAcceleration: 'enabled',
    videoDecode: 'hardware-accelerated',
    scope: 'global',
  });
});

test('normalizes software, unavailable, and unrecognized video decode states', () => {
  const diagnose = (videoDecodeStatus: unknown) => createPlaybackDiagnostics({
    gpuInformationReady: true,
    electronVersion: '43.0.0',
    chromiumVersion: '150.0.0.0',
    hardwareAccelerationEnabled: false,
    videoDecodeStatus,
  });

  const software = diagnose('disabled_software');
  const unavailable = diagnose('unavailable_off');
  const unknown = diagnose('future_status');
  assert.equal(software.available && software.videoDecode, 'software-only');
  assert.equal(unavailable.available && unavailable.videoDecode, 'unavailable');
  assert.equal(unknown.available && unknown.videoDecode, 'unknown');
});

test('does not pass arbitrary version data through the diagnostic response', () => {
  const result = createPlaybackDiagnostics({
    gpuInformationReady: true,
    electronVersion: '43.0.0 C:\\Users\\name',
    chromiumVersion: { secret: true },
    hardwareAccelerationEnabled: true,
    videoDecodeStatus: 'enabled',
  });
  assert.equal(result.available && result.electronVersion, 'unknown');
  assert.equal(result.available && result.chromiumVersion, 'unknown');
});

test('readPlaybackInfo skips a GPU rescan inside the TTL', () => {
  let probes = 0;
  const probe = (): { hardwareAccelerationEnabled: boolean; videoDecodeStatus: unknown } => {
    probes += 1;
    return { hardwareAccelerationEnabled: true, videoDecodeStatus: 'enabled' };
  };
  const first = readPlaybackInfo(true, { electron: '43.0.0', chromium: '150.0.0.0' }, probe, null, 1_000);
  const second = readPlaybackInfo(true, { electron: '43.0.0', chromium: '150.0.0.0' }, probe, first.cache, 20_000);
  const expired = readPlaybackInfo(true, { electron: '43.0.0', chromium: '150.0.0.0' }, probe, second.cache, 40_000);
  assert.equal(probes, 2);
  assert.equal(first.info.available && first.info.videoDecode, 'hardware-accelerated');
  assert.equal(second.info, first.info);
  assert.notEqual(expired.info, first.info);
});

test('readPlaybackInfo probes again once GPU information becomes ready', () => {
  let probes = 0;
  const probe = (): { hardwareAccelerationEnabled: boolean; videoDecodeStatus: unknown } => {
    probes += 1;
    return { hardwareAccelerationEnabled: true, videoDecodeStatus: 'enabled' };
  };
  const initializing = readPlaybackInfo(false, { electron: '43.0.0', chromium: '150.0.0.0' }, probe, null, 1_000);
  const ready = readPlaybackInfo(true, { electron: '43.0.0', chromium: '150.0.0.0' }, probe, initializing.cache, 1_100);
  assert.equal(probes, 1);
  assert.equal(initializing.info.available, false);
  assert.equal(ready.info.available, true);
});
