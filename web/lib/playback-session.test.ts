import assert from 'node:assert/strict';
import test from 'node:test';
import { PlaybackSession, PLAYBACK_STATUS, type PlaybackState } from './playback-session.ts';

class TestVideo extends EventTarget {
  duration = 30;
  paused = true;
  ended = false;
  readyState = 2;
  playbackRate = 1;
  videoWidth = 1920;
  videoHeight = 1080;
  seeks: number[] = [];
  playCalls = 0;
  pendingPlay: Promise<void> | null = null;
  callbacks = new Map<number, VideoFrameRequestCallback>();
  #time = 0;
  #callbackId = 0;
  get currentTime(): number { return this.#time; }
  set currentTime(value: number) { this.#time = value; this.seeks.push(value); }
  play(): Promise<void> {
    this.playCalls += 1;
    this.paused = false;
    this.dispatchEvent(new Event('playing'));
    return this.pendingPlay ?? Promise.resolve();
  }
  pause(): void {
    const changed = !this.paused;
    this.paused = true;
    if (changed) this.dispatchEvent(new Event('pause'));
  }
  requestVideoFrameCallback(callback: VideoFrameRequestCallback): number {
    const id = ++this.#callbackId;
    this.callbacks.set(id, callback);
    return id;
  }
  cancelVideoFrameCallback(id: number): void { this.callbacks.delete(id); }
  frame(seconds: number, count: number): void {
    this.#time = seconds;
    const callbacks = [...this.callbacks.values()];
    this.callbacks.clear();
    for (const callback of callbacks) callback(seconds * 1000, {
      mediaTime: seconds, presentedFrames: count, width: 1920, height: 1080,
      presentationTime: seconds * 1000, expectedDisplayTime: seconds * 1000, processingDuration: 0,
    });
  }
  settle(): void { this.dispatchEvent(new Event('seeked')); }
}

test('plays continuously and fans out actual frames without repeated seeks', async () => {
  const video = new TestVideo();
  const states: PlaybackState[] = [];
  const session = new PlaybackSession(video, { onState: (state) => states.push(state) });
  const left: number[] = [];
  const right: number[] = [];
  session.subscribeFrames((frame) => left.push(frame.seconds));
  session.subscribeFrames((frame) => right.push(frame.seconds));
  session.play();
  await Promise.resolve();
  for (let frame = 1; frame <= 60; frame++) video.frame(frame / 60, frame);
  assert.equal(video.playCalls, 1);
  assert.deepEqual(video.seeks, []);
  assert.equal(left.length, 60);
  assert.deepEqual(left, right);
  assert.ok(states.length < 20, 'React-facing position updates are bounded independently of drawing');
  assert.equal(video.callbacks.size, 1);
  session.dispose();
  assert.equal(video.callbacks.size, 0);
  assert.equal(video.paused, true);
});

test('coalesces seeks and resumes only after the last destination settles', async () => {
  const video = new TestVideo();
  const session = new PlaybackSession(video, { onState: () => {} });
  session.play();
  await Promise.resolve();
  session.seek(3);
  session.seek(9);
  session.seek(12);
  assert.deepEqual(video.seeks, [3]);
  video.settle();
  assert.deepEqual(video.seeks, [3, 12]);
  assert.equal(video.paused, true);
  video.settle();
  await Promise.resolve();
  assert.equal(video.paused, false);
  assert.equal(session.frame.seconds, 12);
  session.dispose();
});

test('pause during a seek cancels resume intent', async () => {
  const video = new TestVideo();
  const session = new PlaybackSession(video, { onState: () => {} });
  session.play();
  await Promise.resolve();
  session.seek(5);
  session.pause();
  video.settle();
  await Promise.resolve();
  assert.equal(video.paused, true);
  assert.equal(video.playCalls, 1);
  assert.equal(session.playRequested, false);
  session.dispose();
});

for (const action of ['pause', 'dispose'] as const) {
  test(`a late play resolution cannot restart a session after ${action}`, async () => {
    const video = new TestVideo();
    let resolvePlay: () => void = () => {};
    video.pendingPlay = new Promise<void>((resolve) => { resolvePlay = resolve; });
    const session = new PlaybackSession(video, { onState: () => {} });
    session.play();
    session[action]();
    resolvePlay();
    await Promise.resolve();
    assert.equal(video.paused, true);
    assert.equal(video.callbacks.size, 0);
    session.dispose();
  });
}

test('range end stops before publishing frames outside the selected cut', async () => {
  const video = new TestVideo();
  let ended = 0;
  const states: PlaybackState[] = [];
  const session = new PlaybackSession(video, { onState: (state) => states.push(state), onRangeEnd: () => { ended += 1; } });
  const frames: number[] = [];
  session.setRange({ start: 0, end: 2, rate: 2 });
  session.subscribeFrames((frame) => frames.push(frame.seconds));
  session.play();
  await Promise.resolve();
  video.frame(1.98, 119);
  video.frame(2.01, 120);
  assert.deepEqual(frames, [1.98]);
  assert.equal(ended, 1);
  assert.equal(video.playbackRate, 2);
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.ended);
  assert.equal(video.paused, true);
  session.dispose();
});

test('an unavailable decoder produces an error and releases playback intent', async () => {
  const video = new TestVideo();
  const states: PlaybackState[] = [];
  video.pendingPlay = Promise.reject(new Error('decode failed'));
  const session = new PlaybackSession(video, { onState: (state) => states.push(state) });
  session.play();
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.error);
  assert.equal(session.playRequested, false);
  session.dispose();
});

test('pending metadata seek stays seeking and stale playing events cannot restart it', () => {
  const video = new TestVideo();
  video.readyState = 0;
  const states: PlaybackState[] = [];
  const session = new PlaybackSession(video, { onState: (state) => states.push(state) });
  session.seek(4);
  video.readyState = 2;
  video.dispatchEvent(new Event('loadedmetadata'));
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.seeking);
  video.dispatchEvent(new Event('playing'));
  assert.equal(video.paused, true);
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.seeking);
  session.pause();
  video.settle();
  video.dispatchEvent(new Event('playing'));
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.paused);
  assert.equal(video.callbacks.size, 0);
  session.dispose();
});

test('buffering stays paused for consumers until playback actually resumes', async () => {
  const video = new TestVideo();
  const states: PlaybackState[] = [];
  const session = new PlaybackSession(video, { onState: (state) => states.push(state) });
  session.play();
  await Promise.resolve();
  video.dispatchEvent(new Event('waiting'));
  video.frame(0.1, 1);
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.buffering);
  video.dispatchEvent(new Event('playing'));
  assert.equal(states.at(-1)?.status, PLAYBACK_STATUS.playing);
  session.dispose();
});

test('scrubbing during playback stays inside the selected range and separates request from presented time', async () => {
  const video = new TestVideo();
  const states: PlaybackState[] = [];
  const session = new PlaybackSession(video, { onState: (state) => states.push(state) });
  session.setRange({ start: 2, end: 5, rate: 1 });
  session.play();
  video.settle();
  await Promise.resolve();
  session.seek(10);
  assert.equal(video.seeks.at(-1), 4.99);
  assert.equal(states.at(-1)?.seconds, 2);
  assert.equal(states.at(-1)?.requestedSeconds, 4.99);
  session.seek(0);
  assert.equal(states.at(-1)?.requestedSeconds, 2);
  video.settle();
  video.settle();
  await Promise.resolve();
  assert.equal(session.frame.seconds, 2);
  assert.equal(states.at(-1)?.requestedSeconds, null);
  session.dispose();
});
