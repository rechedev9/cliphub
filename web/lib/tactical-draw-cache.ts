import type { TacticalGeometry } from './api/tactical.ts';
import type { TimelineEvent } from './tactical-timeline.ts';
import { worldToRendered } from './tactical-transform.ts';
import type { RadarPoint } from './tactical-transform.ts';
import { LruCache } from './lru-cache.ts';

type EventPoints = { at: RadarPoint; target: RadarPoint };

/** Owned by one replay. No sample positions or animation state are cached. */
export class TacticalDrawCache {
  private readonly widths = new LruCache<string, number>(32);
  private points = new WeakMap<TimelineEvent, EventPoints>();
  private geometry: TacticalGeometry | undefined;
  private size = 0;

  clear(): void {
    this.widths.clear();
    this.points = new WeakMap();
  }

  labelWidth(font: string, label: string, measure: () => number): number {
    const key = JSON.stringify([font, label]);
    const cached = this.widths.get(key);
    if (cached !== undefined) return cached;
    const width = measure();
    this.widths.set(key, width);
    return width;
  }

  eventPoints(geometry: TacticalGeometry, size: number, entry: TimelineEvent): EventPoints {
    if (this.geometry !== geometry || this.size !== size) {
      this.points = new WeakMap();
      this.geometry = geometry;
      this.size = size;
    }
    const cached = this.points.get(entry);
    if (cached !== undefined) return cached;
    const event = entry.event;
    const at = worldToRendered(geometry.calibration, event.pos[0], event.pos[1], size);
    const target = event.kind === 'kill'
      ? worldToRendered(geometry.calibration, event.target_pos[0], event.target_pos[1], size)
      : at;
    const points = { at, target };
    this.points.set(entry, points);
    return points;
  }
}
