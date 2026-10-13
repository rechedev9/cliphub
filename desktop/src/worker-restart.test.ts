import test from 'node:test';
import assert from 'node:assert/strict';
import {
  isCloudWorkerMode,
  WORKER_RESTART_LIMITS,
  WorkerRestartPolicy,
  WorkerRestartScheduler,
  workerRestartHint,
} from './worker-restart.ts';

const MINUTE = 60_000;

test('only a Studio with both bridge variables is a cloud worker', () => {
  const cases: Array<{ name: string; env: NodeJS.ProcessEnv; want: boolean }> = [
    { name: 'an ordinary Studio', env: {}, want: false },
    { name: 'the cloud client alone', env: { ZV_CLOUD_URL: 'https://portal.example' }, want: false },
    { name: 'half a bridge', env: { ZV_BRIDGE_URL: 'https://portal.example' }, want: false },
    { name: 'a blank token', env: { ZV_BRIDGE_URL: 'https://portal.example', ZV_BRIDGE_TOKEN: ' ' }, want: false },
    { name: 'the worker', env: { ZV_BRIDGE_URL: 'https://portal.example', ZV_BRIDGE_TOKEN: 'abc' }, want: true },
  ];
  for (const { name, env, want } of cases) {
    assert.equal(isCloudWorkerMode(env), want, name);
  }
});

test('restarts back off from five seconds to five minutes and stop at the cap', () => {
  const policy = new WorkerRestartPolicy();
  const delays: Array<number | null> = [];
  let now = 0;
  for (let failure = 0; failure < WORKER_RESTART_LIMITS.maxRestartsInARow + 2; failure += 1) {
    const delay = policy.nextRestartDelay(now);
    delays.push(delay);
    // Each restart boots and dies again a few seconds later.
    now += (delay ?? 0) + 20_000;
    policy.booted(now - 10_000);
  }
  assert.deepEqual(delays, [
    5_000, 10_000, 20_000, 40_000, 80_000, 160_000, 300_000, 300_000, 300_000, 300_000, null, null,
  ]);
});

test('a backend that stayed up starts the count over', () => {
  const policy = new WorkerRestartPolicy();
  let now = 0;
  for (let failure = 0; failure < 4; failure += 1) {
    policy.nextRestartDelay(now);
    now += 1_000;
  }
  policy.booted(now);

  // It dies again after an hour of work: this is a new problem, not the loop.
  assert.equal(policy.nextRestartDelay(now + 60 * MINUTE), 5_000);
  // That restart never finished booting, so the next one waits longer.
  assert.equal(policy.nextRestartDelay(now + 61 * MINUTE), 10_000);
});

test('a backend that died again within minutes keeps backing off', () => {
  const policy = new WorkerRestartPolicy();
  assert.equal(policy.nextRestartDelay(0), 5_000);
  policy.booted(10_000);
  assert.equal(policy.nextRestartDelay(10_000 + 9 * MINUTE), 10_000);
});

test('a manual retry starts the count over, also after the cap', () => {
  const policy = new WorkerRestartPolicy();
  for (let failure = 0; failure < WORKER_RESTART_LIMITS.maxRestartsInARow; failure += 1) {
    policy.nextRestartDelay(failure);
  }
  assert.equal(policy.nextRestartDelay(100), null);
  policy.reset();
  assert.equal(policy.nextRestartDelay(200), 5_000);
});

test('the stop screen says when the restart comes, or that it will not', () => {
  assert.equal(
    workerRestartHint(40_000),
    'Este equipo trabaja para la nube: ClipHub Studio se reinicia solo en 40 s.',
  );
  assert.equal(
    workerRestartHint(null),
    'Este equipo trabaja para la nube, pero dejó de reiniciarse solo tras 10 intentos seguidos. Revisa el registro y pulsa Reintentar.',
  );
});

interface FakeTimer {
  delayMs: number;
  fire: () => void;
  canceled: boolean;
}

/** A scheduler on fake timers, with a backend whose restarts the test scripts. */
function harness(options: { enabled: boolean; restartResults?: boolean[] }) {
  const timers: FakeTimer[] = [];
  const log: string[] = [];
  const restartResults = options.restartResults ?? [];
  let restarts = 0;
  let now = 0;
  const scheduler = new WorkerRestartScheduler({
    enabled: options.enabled,
    restart: () => {
      restarts += 1;
      return restartResults.shift() ?? true;
    },
    logLine: (text) => log.push(text),
    now: () => now,
    startTimer: (callback, delayMs) => {
      const timer: FakeTimer = { delayMs, fire: callback, canceled: false };
      timers.push(timer);
      return { cancel: () => { timer.canceled = true; } };
    },
  });
  return {
    scheduler,
    timers,
    log,
    restarts: () => restarts,
    advance: (ms: number) => { now += ms; },
    pending: () => timers.filter((timer) => !timer.canceled),
  };
}

test('an ordinary Studio never restarts by itself and keeps its own stop screen', () => {
  const studio = harness({ enabled: false });
  studio.scheduler.backendBooted();

  assert.equal(studio.scheduler.backendStopped(), undefined);

  assert.equal(studio.timers.length, 0);
  assert.equal(studio.restarts(), 0);
  assert.deepEqual(studio.log, []);
});

test('a cloud worker restarts its backend after it stopped', () => {
  const worker = harness({ enabled: true });
  worker.scheduler.backendBooted();
  worker.advance(60_000);

  const hint = worker.scheduler.backendStopped();

  assert.equal(hint, 'Este equipo trabaja para la nube: ClipHub Studio se reinicia solo en 5 s.');
  assert.equal(worker.restarts(), 0, 'it waits for the delay first');
  assert.deepEqual(worker.timers.map((timer) => timer.delayMs), [5_000]);
  worker.timers[0].fire();
  assert.equal(worker.restarts(), 1);
  assert.deepEqual(worker.log, ['[boot] cloud worker: restarting the backend in 5 s\n']);
});

test('a restart that had to be deferred is tried again, later', () => {
  const worker = harness({ enabled: true, restartResults: [false, true] });
  worker.scheduler.backendStopped();

  worker.timers[0].fire();

  assert.equal(worker.restarts(), 1);
  assert.deepEqual(worker.timers.map((timer) => timer.delayMs), [5_000, 10_000]);
  worker.timers[1].fire();
  assert.equal(worker.restarts(), 2);
  assert.equal(worker.timers.length, 2, 'a restart that started plans nothing more');
});

test('two stops plan one restart, not two', () => {
  const worker = harness({ enabled: true });
  worker.scheduler.backendStopped();
  worker.scheduler.backendStopped();

  assert.deepEqual(worker.pending().map((timer) => timer.delayMs), [10_000]);
});

test('pressing Reintentar replaces the automatic restart and starts the count over', () => {
  const worker = harness({ enabled: true });
  worker.scheduler.backendStopped();
  worker.scheduler.backendStopped();

  worker.scheduler.manualRetry();

  assert.equal(worker.pending().length, 0);
  assert.equal(
    worker.scheduler.backendStopped(),
    'Este equipo trabaja para la nube: ClipHub Studio se reinicia solo en 5 s.',
  );
});

test('after too many failures in a row the worker stops and says so', () => {
  const worker = harness({ enabled: true });
  let hint: string | undefined;
  for (let failure = 0; failure <= WORKER_RESTART_LIMITS.maxRestartsInARow; failure += 1) {
    hint = worker.scheduler.backendStopped();
  }

  assert.equal(hint, workerRestartHint(null));
  assert.equal(worker.pending().length, 0);
  assert.equal(worker.timers.length, WORKER_RESTART_LIMITS.maxRestartsInARow);
  assert.equal(worker.log.at(-1), '[boot] cloud worker: automatic restarts stopped after too many failures in a row\n');
});

test('nothing restarts once the app is quitting', () => {
  const worker = harness({ enabled: true });
  worker.scheduler.backendStopped();

  worker.scheduler.dispose();

  assert.equal(worker.pending().length, 0);
  assert.equal(worker.scheduler.backendStopped(), undefined);
  // A timer that already fired its callback must not boot either.
  worker.timers[0].fire();
  assert.equal(worker.restarts(), 0);
});
