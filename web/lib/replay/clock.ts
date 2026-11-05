/** A playhead listener; it runs on every animation frame while the replay plays. */
export type ClockListener = (seconds: number) => void;

/**
 * The replay's playhead, shared by the transport and every view that draws it.
 * It lives outside React state so playback never re-renders the page per frame.
 */
export class ReplayClock {
  #seconds = 0;
  readonly #listeners = new Set<ClockListener>();

  get seconds(): number {
    return this.#seconds;
  }

  /** Moves the playhead; listeners hear only a real change. */
  set(seconds: number): void {
    const next = Number.isFinite(seconds) ? seconds : 0;
    if (next === this.#seconds) return;
    this.#seconds = next;
    for (const listener of this.#listeners) listener(next);
  }

  /** Notifies listeners again without moving, for a view whose inputs changed. */
  refresh(): void {
    for (const listener of this.#listeners) listener(this.#seconds);
  }

  subscribe(listener: ClockListener): () => void {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  }
}
