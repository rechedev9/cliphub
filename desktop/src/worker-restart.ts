import { bridgeEnvironment } from './bridge-environment.ts';

/**
 * A Studio with both bridge variables set is the unattended cloud worker: no
 * one sits in front of it, so it has to bring its own backend back.
 */
export function isCloudWorkerMode(env: NodeJS.ProcessEnv): boolean {
  return Object.keys(bridgeEnvironment(env)).length > 0;
}

export interface WorkerRestartLimits {
  /** Delay before the first automatic restart; it doubles on each failure in a row. */
  baseDelayMs: number;
  maxDelayMs: number;
  /** Failures in a row after which the worker stops and waits for a person. */
  maxRestartsInARow: number;
  /** A backend that stayed up this long was healthy, so the count starts over. */
  stableAfterMs: number;
}

export const WORKER_RESTART_LIMITS: WorkerRestartLimits = {
  baseDelayMs: 5_000,
  maxDelayMs: 5 * 60_000,
  maxRestartsInARow: 10,
  stableAfterMs: 10 * 60_000,
};

/**
 * Decides when a cloud worker restarts its backend after it stopped. The
 * backoff keeps a crash loop from hammering the machine and the portal, and
 * the cap ends a loop that no restart is going to fix.
 */
export class WorkerRestartPolicy {
  private readonly limits: WorkerRestartLimits;
  private failuresInARow = 0;
  private bootedAt: number | null = null;

  constructor(limits: WorkerRestartLimits = WORKER_RESTART_LIMITS) {
    this.limits = limits;
  }

  /** The backend finished booting. */
  booted(now: number): void {
    this.bootedAt = now;
  }

  /**
   * The backend stopped or failed to boot. Returns how long to wait before
   * restarting it, or null once the cap is reached.
   */
  nextRestartDelay(now: number): number | null {
    if (this.bootedAt !== null && now - this.bootedAt >= this.limits.stableAfterMs) {
      this.failuresInARow = 0;
    }
    this.bootedAt = null;
    if (this.failuresInARow >= this.limits.maxRestartsInARow) return null;
    const delay = Math.min(this.limits.baseDelayMs * 2 ** this.failuresInARow, this.limits.maxDelayMs);
    this.failuresInARow += 1;
    return delay;
  }

  /** A person restarted the backend, which starts the count over. */
  reset(): void {
    this.failuresInARow = 0;
  }
}

/** The sentence the stop screen adds on a cloud worker. */
export function workerRestartHint(delayMs: number | null, limits: WorkerRestartLimits = WORKER_RESTART_LIMITS): string {
  if (delayMs === null) {
    return `Este equipo trabaja para la nube, pero dejó de reiniciarse solo tras ${limits.maxRestartsInARow} intentos seguidos. Revisa el registro y pulsa Reintentar.`;
  }
  return `Este equipo trabaja para la nube: ClipHub Studio se reinicia solo en ${Math.ceil(delayMs / 1000)} s.`;
}

export interface RestartTimer {
  cancel(): void;
}

export interface WorkerRestartSchedulerOptions {
  /** False on an ordinary Studio, which never restarts anything by itself. */
  enabled: boolean;
  /** Boots the backend again. Returns false when the boot had to be deferred. */
  restart: () => boolean;
  logLine: (text: string) => void;
  limits?: WorkerRestartLimits;
  now?: () => number;
  startTimer?: (callback: () => void, delayMs: number) => RestartTimer;
}

function startNodeTimer(callback: () => void, delayMs: number): RestartTimer {
  const timer = setTimeout(callback, delayMs);
  return { cancel: () => clearTimeout(timer) };
}

/**
 * Brings the backend of a cloud worker back after it stopped, without anyone
 * pressing Reintentar. On an ordinary Studio it does nothing at all.
 */
export class WorkerRestartScheduler {
  private readonly enabled: boolean;
  private readonly restart: () => boolean;
  private readonly logLine: (text: string) => void;
  private readonly limits: WorkerRestartLimits;
  private readonly now: () => number;
  private readonly startTimer: (callback: () => void, delayMs: number) => RestartTimer;
  private readonly policy: WorkerRestartPolicy;
  private pending: RestartTimer | null = null;
  private disposed = false;

  constructor(options: WorkerRestartSchedulerOptions) {
    this.enabled = options.enabled;
    this.restart = options.restart;
    this.logLine = options.logLine;
    this.limits = options.limits ?? WORKER_RESTART_LIMITS;
    this.now = options.now ?? Date.now;
    this.startTimer = options.startTimer ?? startNodeTimer;
    this.policy = new WorkerRestartPolicy(this.limits);
  }

  backendBooted(): void {
    this.policy.booted(this.now());
  }

  /**
   * The backend stopped or failed to boot. Plans its restart and returns what
   * the stop screen says about it; undefined on an ordinary Studio.
   */
  backendStopped(): string | undefined {
    if (!this.enabled || this.disposed) return undefined;
    this.cancelPending();
    const delayMs = this.policy.nextRestartDelay(this.now());
    if (delayMs === null) {
      this.logLine('[boot] cloud worker: automatic restarts stopped after too many failures in a row\n');
      return workerRestartHint(null, this.limits);
    }
    this.logLine(`[boot] cloud worker: restarting the backend in ${Math.ceil(delayMs / 1000)} s\n`);
    this.pending = this.startTimer(() => {
      this.pending = null;
      if (this.disposed) return;
      // A boot that had to be deferred counts as one more failure.
      if (!this.restart()) this.backendStopped();
    }, delayMs);
    return workerRestartHint(delayMs, this.limits);
  }

  /** A person is restarting the backend: no automatic restart on top, and the count starts over. */
  manualRetry(): void {
    this.cancelPending();
    this.policy.reset();
  }

  /** The app is quitting. */
  dispose(): void {
    this.disposed = true;
    this.cancelPending();
  }

  private cancelPending(): void {
    this.pending?.cancel();
    this.pending = null;
  }
}
