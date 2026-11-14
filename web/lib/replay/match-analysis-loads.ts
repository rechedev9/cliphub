export type MatchAnalysis<T> = { state: 'loading' } | { state: 'ready'; analysis: T } | { state: 'failed'; message: string };

export type MatchAnalysisLoadsOptions<T> = {
  load: (id: string) => Promise<T>;
  message: (error: unknown) => string;
  onChange: (states: ReadonlyMap<string, MatchAnalysis<T>>) => void;
  seed?: { id: string; analysis: T };
};

/**
 * Tactical analyses by match: each one is requested the first time it is wanted
 * or prefetched and kept for the rest of the visit.
 */
export class MatchAnalysisLoads<T> {
  states: ReadonlyMap<string, MatchAnalysis<T>>;
  private readonly options: MatchAnalysisLoadsOptions<T>;

  constructor(options: MatchAnalysisLoadsOptions<T>) {
    this.options = options;
    const { seed } = options;
    this.states = seed === undefined ? new Map() : new Map([[seed.id, { state: 'ready', analysis: seed.analysis }]]);
  }

  want(ids: readonly string[]): void {
    for (const id of ids) if (!this.states.has(id)) this.start(id);
  }

  prefetch(id: string): void {
    this.want([id]);
  }

  retry(id: string): void {
    if (this.states.get(id)?.state === 'failed') this.start(id);
  }

  seed(id: string, analysis: T): void {
    const existing = this.states.get(id);
    if (existing?.state === 'ready' && existing.analysis === analysis) return;
    this.set(id, { state: 'ready', analysis });
  }

  private start(id: string): void {
    this.set(id, { state: 'loading' });
    this.options.load(id).then(
      (analysis) => this.set(id, { state: 'ready', analysis }),
      (error: unknown) => this.set(id, { state: 'failed', message: this.options.message(error) }),
    );
  }

  private set(id: string, state: MatchAnalysis<T>): void {
    this.states = new Map(this.states).set(id, state);
    this.options.onChange(this.states);
  }
}
