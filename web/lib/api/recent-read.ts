/**
 * One read shared by every caller while it is open and for `keepMs` after it
 * succeeds, so polls that tick together send one request. A failed read is
 * never reused, and `drop` forgets the last one after a write.
 */
export class RecentRead<T> {
  private open: Promise<T> | null = null;
  private settledAt: number | null = null;
  private readonly read: () => Promise<T>;
  private readonly keepMs: number;
  private readonly now: () => number;

  constructor(read: () => Promise<T>, options: { keepMs: number; now?: () => number }) {
    this.read = read;
    this.keepMs = options.keepMs;
    this.now = options.now ?? Date.now;
  }

  get(): Promise<T> {
    const open = this.open;
    if (open !== null && (this.settledAt === null || this.now() - this.settledAt < this.keepMs)) return open;
    const started = this.read();
    this.open = started;
    this.settledAt = null;
    started.then(
      () => {
        if (this.open === started) this.settledAt = this.now();
      },
      () => {
        if (this.open === started) this.drop();
      },
    );
    return started;
  }

  drop(): void {
    this.open = null;
    this.settledAt = null;
  }
}
