// A render loop can throw the same error hundreds of times a second. Each one
// would cost a synchronous queue write in Electron main and could evict the
// pipeline.error that matters from the 200-event queue.

export const RENDERER_ERROR_REPEAT_MS = 60_000;
export const RENDERER_ERRORS_PER_SESSION = 20;

export interface RendererErrorGuardOptions {
  now?: () => number;
  repeatMS?: number;
  maxPerSession?: number;
}

/** Identical messages go out at most once per window, and a page session sends at most a fixed number. */
export class RendererErrorGuard {
  private readonly now: () => number;
  private readonly repeatMS: number;
  private readonly maxPerSession: number;
  private readonly lastSent = new Map<string, number>();
  private sent = 0;

  constructor(options: RendererErrorGuardOptions = {}) {
    this.now = options.now ?? Date.now;
    this.repeatMS = options.repeatMS ?? RENDERER_ERROR_REPEAT_MS;
    this.maxPerSession = options.maxPerSession ?? RENDERER_ERRORS_PER_SESSION;
  }

  allow(message: string): boolean {
    if (this.sent >= this.maxPerSession) return false;
    const now = this.now();
    const last = this.lastSent.get(message);
    if (last !== undefined && now - last < this.repeatMS) return false;
    this.lastSent.set(message, now);
    this.sent++;
    return true;
  }
}

/** The dedup key: stacks differ between call paths of the same failure. */
export function rendererErrorKey(error: Error): string {
  return `${error.name}: ${error.message}`;
}
