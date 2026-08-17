export const EDITOR_SCHEMA_VERSION = '1.0' as const;
export const EDITOR_FILTERS = { none: '', grade: 'grade' } as const;
export const EDITOR_TRACK_KINDS = { video: 'video', audio: 'audio' } as const;
export const EDITOR_CANVAS = {
  portrait: { width: 1080, height: 1920, fps: 60 },
  landscape: { width: 1920, height: 1080, fps: 60 },
} as const;

export type EditorFilter = (typeof EDITOR_FILTERS)[keyof typeof EDITOR_FILTERS];
export type EditorTrackKind = (typeof EDITOR_TRACK_KINDS)[keyof typeof EDITOR_TRACK_KINDS];

export type EditorTransform = {
  x: number;
  y: number;
  width: number;
  height: number;
  opacity?: number;
};

export type EditorItem = {
  id: string;
  asset_id: string;
  timeline_start: number;
  source_in: number;
  source_out: number;
  speed?: number;
  volume?: number;
  fade_in?: number;
  fade_out?: number;
  transform?: EditorTransform;
  filter?: EditorFilter;
};

export type EditorTrack = {
  id: string;
  kind: EditorTrackKind;
  items: EditorItem[];
};

export type EditorTextOverlay = {
  id: string;
  text: string;
  position_y: number;
  start_seconds: number;
  end_seconds?: number;
  font_size?: number;
};

export type EditorDocument = {
  schema_version: string;
  canvas: { width: number; height: number; fps: number };
  tracks: EditorTrack[];
  overlays?: EditorTextOverlay[];
  music?: { key?: string; volume?: number };
};

export type EditorLayer = {
  item_id: string;
  track_id: string;
  asset_id: string;
  source_time: number;
  transform: EditorTransform;
  opacity: number;
  filter: EditorFilter;
};

export type EditorTextSample = {
  id: string;
  text: string;
  position_y: number;
  font_size: number;
};

export type EditorSample = {
  time: number;
  duration: number;
  layers: EditorLayer[];
  texts: EditorTextSample[];
};

export function itemSpeed(item: EditorItem): number {
  return item.speed && item.speed !== 0 ? item.speed : 1;
}

export function itemOutputDuration(item: EditorItem): number {
  return (item.source_out - item.source_in) / itemSpeed(item);
}

export function itemTimelineEnd(item: EditorItem): number {
  return item.timeline_start + itemOutputDuration(item);
}

export function documentDuration(doc: EditorDocument): number {
  let end = 0;
  for (const track of doc.tracks) {
    for (const item of track.items) {
      const itemEnd = itemTimelineEnd(item);
      if (itemEnd > end) end = itemEnd;
    }
  }
  return end;
}

export function resolvedTransform(item: EditorItem): EditorTransform {
  if (item.transform === undefined) {
    return { x: 0, y: 0, width: 1, height: 1, opacity: 1 };
  }
  return {
    ...item.transform,
    opacity: item.transform.opacity && item.transform.opacity !== 0 ? item.transform.opacity : 1,
  };
}

function fadeOpacity(local: number, duration: number, fadeIn: number, fadeOut: number): number {
  let opacity = 1;
  if (fadeIn > 0 && local < fadeIn) opacity = local / fadeIn;
  if (fadeOut > 0 && local > duration - fadeOut) {
    const tail = (duration - local) / fadeOut;
    if (tail < opacity) opacity = tail;
  }
  if (opacity < 0) return 0;
  if (opacity > 1) return 1;
  return opacity;
}

export function evaluateTimeline(doc: EditorDocument, time: number): EditorSample {
  const duration = documentDuration(doc);
  const sample: EditorSample = { time, duration, layers: [], texts: [] };
  if (time < 0 || time > duration) return sample;
  for (const track of doc.tracks) {
    if (track.kind !== EDITOR_TRACK_KINDS.video) continue;
    for (const item of track.items) {
      if (time < item.timeline_start || time >= itemTimelineEnd(item)) continue;
      const local = time - item.timeline_start;
      const base = resolvedTransform(item);
      const opacity = (base.opacity ?? 1) * fadeOpacity(local, itemOutputDuration(item), item.fade_in ?? 0, item.fade_out ?? 0);
      if (opacity <= 0) continue;
      sample.layers.push({
        item_id: item.id,
        track_id: track.id,
        asset_id: item.asset_id,
        source_time: item.source_in + local * itemSpeed(item),
        transform: { ...base, opacity },
        opacity,
        filter: item.filter ?? EDITOR_FILTERS.none,
      });
    }
  }
  for (const overlay of doc.overlays ?? []) {
    const end = overlay.end_seconds ?? duration;
    if (time < overlay.start_seconds || time >= end) continue;
    sample.texts.push({
      id: overlay.id,
      text: overlay.text,
      position_y: overlay.position_y,
      font_size: overlay.font_size && overlay.font_size !== 0 ? overlay.font_size : 64,
    });
  }
  return sample;
}

export function defaultEditorDocument(): EditorDocument {
  return {
    schema_version: EDITOR_SCHEMA_VERSION,
    canvas: { ...EDITOR_CANVAS.portrait },
    tracks: [{ id: 'v1', kind: EDITOR_TRACK_KINDS.video, items: [] }],
    overlays: [],
  };
}
