import assert from 'node:assert/strict';
import test from 'node:test';
import { StreamAudioMixer } from './stream-audio.ts';
import type { StreamClipRange } from './api/streams.ts';

class FakeAudioParam {
  value = 1;
  setValueAtTime(value: number): void {
    this.value = value;
  }
}

class FakeGain {
  readonly gain = new FakeAudioParam();
  disconnected = false;
  connect(): void {}
  disconnect(): void { this.disconnected = true; }
}

class FakeSource {
  disconnected = false;
  connect(): void {}
  disconnect(): void { this.disconnected = true; }
}

class FakeAudioContext {
  static instances: FakeAudioContext[] = [];
  readonly destination = {};
  readonly gains: FakeGain[] = [];
  readonly sources: unknown[] = [];
  currentTime = 12;
  resumeCalls = 0;
  closeCalls = 0;

  constructor() { FakeAudioContext.instances.push(this); }
  createMediaElementSource(element: unknown): FakeSource {
    this.sources.push(element);
    return new FakeSource();
  }
  createGain(): FakeGain {
    const gain = new FakeGain();
    this.gains.push(gain);
    return gain;
  }
  async resume(): Promise<void> { this.resumeCalls += 1; }
  async close(): Promise<void> { this.closeCalls += 1; }
}

class FakeAudio {
  static instances: FakeAudio[] = [];
  private time = 0;
  src = '';
  loop = false;
  preload = '';
  playbackRate = 1;
  duration = 10;
  paused = true;
  pauseCalls = 0;
  playCalls = 0;
  seekWrites = 0;
  loadCalls = 0;
  onerror: (() => void) | null = null;
  playResult: Promise<void> = Promise.resolve();

  constructor() { FakeAudio.instances.push(this); }
  get currentTime(): number { return this.time; }
  set currentTime(value: number) { this.time = value; this.seekWrites += 1; }
  pause(): void { this.paused = true; this.pauseCalls += 1; }
  play(): Promise<void> { this.paused = false; this.playCalls += 1; return this.playResult; }
  removeAttribute(name: string): void { if (name === 'src') this.src = ''; }
  load(): void { this.loadCalls += 1; }
}

function installWebAudio(): () => void {
  const audioContext = Object.getOwnPropertyDescriptor(globalThis, 'AudioContext');
  const audio = Object.getOwnPropertyDescriptor(globalThis, 'Audio');
  FakeAudioContext.instances = [];
  FakeAudio.instances = [];
  Object.defineProperty(globalThis, 'AudioContext', { configurable: true, value: FakeAudioContext });
  Object.defineProperty(globalThis, 'Audio', { configurable: true, value: FakeAudio });
  return () => {
    if (audioContext) Object.defineProperty(globalThis, 'AudioContext', audioContext);
    else delete (globalThis as { AudioContext?: unknown }).AudioContext;
    if (audio) Object.defineProperty(globalThis, 'Audio', audio);
    else delete (globalThis as { Audio?: unknown }).Audio;
  };
}

function clip(overrides: Partial<StreamClipRange> = {}): StreamClipRange {
  return {
    id: 'clip-1',
    start_seconds: 10,
    end_seconds: 18,
    edit: { speed: 2, source_volume: 2, fade_in_seconds: 1, fade_out_seconds: 1 },
    ...overrides,
  };
}

test('stays on direct video audio until resume and resumes without music', async () => {
  const restore = installWebAudio();
  try {
    const video = {} as HTMLVideoElement;
    const mixer = new StreamAudioMixer(video, () => assert.fail('unexpected error'));
    mixer.configure(null, null);
    mixer.sync(7, true);
    assert.equal(FakeAudioContext.instances.length, 0);

    await mixer.resume();
    const context = FakeAudioContext.instances[0];
    assert.equal(context.resumeCalls, 1);
    assert.deepEqual(context.sources, [video]);
    assert.equal(context.gains[0].gain.value, 1);
    assert.equal(FakeAudio.instances.length, 0);
    mixer.dispose();
    await Promise.resolve();
    assert.equal(context.closeCalls, 1);
  } finally {
    restore();
  }
});

test('matches render gain, default music volume, speed, fades, and loop timing', async () => {
  const restore = installWebAudio();
  try {
    const mixer = new StreamAudioMixer({} as HTMLVideoElement, () => assert.fail('unexpected error'));
    mixer.configure(clip(), { url: '/music/track.mp3', volume: 0 });
    await mixer.resume();
    const context = FakeAudioContext.instances[0];
    const music = FakeAudio.instances[0];

    mixer.sync(11, false); // 1 source second / 2x = 0.5 output seconds.
    assert.equal(context.gains[0].gain.value, 1); // source 2.0 * fade 0.5
    assert.equal(context.gains[1].gain.value, 0.125); // music default 0.25 * fade 0.5
    assert.equal(music.currentTime, 0.5);
    assert.equal(music.playbackRate, 1);

    music.duration = 1.5;
    mixer.sync(17, false); // 3.5 output seconds loops to 0.5 and is halfway through fade-out.
    assert.equal(context.gains[0].gain.value, 1);
    assert.equal(context.gains[1].gain.value, 0.125);
    assert.equal(music.currentTime, 0.5);
  } finally {
    restore();
  }
});

test('multiplies overlapping fade gains like cascaded FFmpeg afade filters', async () => {
  const restore = installWebAudio();
  try {
    const mixer = new StreamAudioMixer({} as HTMLVideoElement, () => assert.fail('unexpected error'));
    mixer.configure(clip({
      start_seconds: 0,
      end_seconds: 4,
      edit: { source_volume: 1, fade_in_seconds: 3, fade_out_seconds: 3 },
    }), { url: '/music/track.mp3', volume: 0.5 });
    await mixer.resume();
    mixer.sync(2, false);
    const context = FakeAudioContext.instances[0];
    assert.ok(Math.abs(context.gains[0].gain.value - 4 / 9) < 0.000001);
    assert.ok(Math.abs(context.gains[1].gain.value - 2 / 9) < 0.000001);
  } finally {
    restore();
  }
});

test('seeks music only for configuration changes or meaningful drift', async () => {
  const restore = installWebAudio();
  try {
    const mixer = new StreamAudioMixer({} as HTMLVideoElement, () => assert.fail('unexpected error'));
    mixer.configure(clip({ edit: undefined }), { url: '/music/track.mp3', volume: 0.4 });
    await mixer.resume();
    const music = FakeAudio.instances[0];
    mixer.sync(10, false);
    const initialWrites = music.seekWrites;

    music.currentTime = 1.1;
    const externalWrite = music.seekWrites;
    mixer.sync(11, false);
    assert.equal(music.seekWrites, externalWrite); // 100 ms drift stays untouched.

    music.currentTime = 0;
    mixer.sync(11, false);
    assert.equal(music.seekWrites, externalWrite + 2); // test write plus one drift correction.
    assert.equal(music.currentTime, 1);

    mixer.configure(clip({ id: 'clip-2', start_seconds: 20, end_seconds: 25, edit: undefined }), { url: '/music/track.mp3', volume: 0.4 });
    assert.equal(music.currentTime, 0);
    mixer.sync(20, false);
    assert.ok(music.seekWrites > initialWrites);
    assert.equal(music.currentTime, 0);
    assert.equal(FakeAudio.instances.length, 1);
  } finally {
    restore();
  }
});

test('controls music playback and suppresses late errors after disposal', async () => {
  const restore = installWebAudio();
  try {
    let rejectPlay: (reason: Error) => void = () => {};
    let errors = 0;
    const mixer = new StreamAudioMixer({} as HTMLVideoElement, () => { errors += 1; });
    mixer.configure(clip({ edit: undefined }), { url: '/music/track.mp3', volume: 0.5 });
    await mixer.resume();
    const music = FakeAudio.instances[0];
    music.playResult = new Promise((_, reject) => { rejectPlay = reject; });

    mixer.sync(10, true);
    assert.equal(music.playCalls, 1);
    mixer.sync(10.1, true);
    assert.equal(music.playCalls, 1);
    mixer.pause();
    assert.equal(music.paused, true);
    rejectPlay(new Error('late play failure'));
    await Promise.resolve();
    await Promise.resolve();
    assert.equal(errors, 0);
    mixer.dispose();
    assert.equal(music.src, '');
    assert.ok(music.loadCalls > 0);
  } finally {
    restore();
  }
});

test('reports current music element failures without treating silent source video as an error', async () => {
  const restore = installWebAudio();
  try {
    let errors = 0;
    const mixer = new StreamAudioMixer({} as HTMLVideoElement, () => { errors += 1; });
    mixer.configure(clip({ edit: { source_volume: 0 } }), { url: '/music/track.mp3', volume: 0.5 });
    await mixer.resume();
    const context = FakeAudioContext.instances[0];
    const music = FakeAudio.instances[0];
    mixer.sync(10, false);
    assert.equal(context.gains[0].gain.value, 0);
    assert.equal(errors, 0);
    music.onerror?.();
    assert.equal(errors, 1);
  } finally {
    restore();
  }
});
