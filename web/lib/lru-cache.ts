/** Small, instance-owned cache. Reads promote entries; eviction releases references. */
export class LruCache<K, V> {
  private readonly entries = new Map<K, V>();
  private readonly capacity: number;

  constructor(capacity: number) {
    if (!Number.isSafeInteger(capacity) || capacity < 1) throw new RangeError('cache capacity must be a positive integer');
    this.capacity = capacity;
  }

  get size(): number { return this.entries.size; }

  get(key: K): V | undefined {
    if (!this.entries.has(key)) return undefined;
    const value = this.entries.get(key) as V;
    this.entries.delete(key);
    this.entries.set(key, value);
    return value;
  }

  set(key: K, value: V): void {
    this.entries.delete(key);
    this.entries.set(key, value);
    if (this.entries.size > this.capacity) {
      this.entries.delete(this.entries.keys().next().value as K);
    }
  }

  clear(): void { this.entries.clear(); }
}
