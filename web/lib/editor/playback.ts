import { itemSpeed, itemTimelineEnd, type EditorDocument, type EditorItem } from './evaluate.ts';

export function upcomingItems(doc: EditorDocument, time: number, horizon: number): EditorItem[] {
  const windowEnd = time + horizon;
  const items: EditorItem[] = [];
  for (const track of doc.tracks ?? []) {
    for (const item of track.items ?? []) {
      if (item.timeline_start <= windowEnd && itemTimelineEnd(item) > time) {
        items.push(item);
      }
    }
  }
  return items;
}

export function clampVolumeForPreview(volume: number | undefined): number {
  if (volume === undefined || !Number.isFinite(volume)) return 1;
  if (volume < 0) return 0;
  if (volume > 1) return 1;
  return volume;
}

export function itemPlaybackRate(item: EditorItem): number {
  return itemSpeed(item);
}
