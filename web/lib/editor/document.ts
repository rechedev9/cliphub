import {
  EDITOR_CANVAS,
  EDITOR_TRACK_KINDS,
  documentDuration,
  itemSpeed,
  itemTimelineEnd,
  type EditorDocument,
  type EditorItem,
  type EditorTextOverlay,
  type EditorTrack,
  type EditorTrackKind,
  type EditorTransition,
  type EditorTransitionKind,
} from './evaluate.ts';
import { EDITOR_LIMITS } from './validate.ts';

export type EditorAssetRef = { id: string; probe: { duration_seconds?: number } };

export type EditorItemPatch = Partial<
  Pick<EditorItem, 'speed' | 'volume' | 'fade_in' | 'fade_out' | 'transform' | 'filter' | 'timeline_start' | 'source_in' | 'source_out'>
>;

function tracksOf(doc: EditorDocument): EditorTrack[] {
  return doc.tracks ?? [];
}

function itemIds(doc: EditorDocument): Set<string> {
  const ids = new Set<string>();
  for (const track of tracksOf(doc)) {
    for (const item of track.items ?? []) ids.add(item.id);
  }
  return ids;
}

function nextPrefixedId(prefix: string, used: Set<string>): string {
  let n = 1;
  let id = `${prefix}${n.toString(36)}`;
  while (used.has(id)) {
    n += 1;
    id = `${prefix}${n.toString(36)}`;
  }
  return id;
}

function nextClipId(doc: EditorDocument): string {
  return nextPrefixedId('clip-', itemIds(doc));
}

function findItem(doc: EditorDocument, itemId: string): { track: EditorTrack; item: EditorItem } | undefined {
  for (const track of tracksOf(doc)) {
    for (const item of track.items ?? []) {
      if (item.id === itemId) return { track, item };
    }
  }
  return undefined;
}

function mapItem(doc: EditorDocument, itemId: string, update: (item: EditorItem) => EditorItem): EditorDocument {
  let found = false;
  const tracks = tracksOf(doc).map((track) => {
    const items = (track.items ?? []).map((item) => {
      if (item.id !== itemId) return item;
      found = true;
      return update(item);
    });
    return found && items !== track.items ? { ...track, items } : track;
  });
  return found ? { ...doc, tracks } : doc;
}

function nextTrackId(doc: EditorDocument, kind: EditorTrackKind): string {
  const prefix = kind === EDITOR_TRACK_KINDS.video ? 'v' : 'a';
  const pattern = new RegExp(`^${prefix}(\\d+)$`);
  let max = 0;
  for (const track of tracksOf(doc)) {
    const match = pattern.exec(track.id);
    const n = match === null ? Number.NaN : Number(match[1]);
    if (Number.isInteger(n) && n > max) max = n;
  }
  return `${prefix}${max + 1}`;
}

export function addItem(doc: EditorDocument, trackId: string, asset: EditorAssetRef, at?: number): EditorDocument {
  const tracks = tracksOf(doc);
  const track = tracks.find((candidate) => candidate.id === trackId);
  if (track === undefined) return doc;
  const items = track.items ?? [];
  if (items.length >= EDITOR_LIMITS.maxItemsPerTrack) return doc;
  const item: EditorItem = {
    id: nextClipId(doc),
    asset_id: asset.id,
    timeline_start: at ?? documentDuration(doc),
    source_in: 0,
    source_out: Math.max(0.2, asset.probe.duration_seconds ?? 2),
  };
  return {
    ...doc,
    tracks: tracks.map((candidate) => (candidate.id === trackId ? { ...candidate, items: [...items, item] } : candidate)),
  };
}

export function moveItem(doc: EditorDocument, itemId: string, timelineStart: number): EditorDocument {
  if (findItem(doc, itemId) === undefined) return doc;
  const start = timelineStart < 0 ? 0 : timelineStart;
  return mapItem(doc, itemId, (item) => ({ ...item, timeline_start: start }));
}

export function trimItem(
  doc: EditorDocument,
  itemId: string,
  sourceIn: number,
  sourceOut: number,
  assetDuration?: number,
): EditorDocument {
  if (findItem(doc, itemId) === undefined) return doc;
  let nextIn = sourceIn < 0 ? 0 : sourceIn;
  let nextOut = sourceOut;
  if (assetDuration !== undefined) {
    if (nextIn > assetDuration) nextIn = assetDuration;
    if (nextOut > assetDuration) nextOut = assetDuration;
  }
  if (nextOut <= nextIn) return doc;
  return mapItem(doc, itemId, (item) => ({ ...item, source_in: nextIn, source_out: nextOut }));
}

export function splitItemAt(doc: EditorDocument, itemId: string, timelineTime: number): EditorDocument {
  const found = findItem(doc, itemId);
  if (found === undefined) return doc;
  const start = found.item.timeline_start;
  const end = itemTimelineEnd(found.item);
  if (timelineTime <= start || timelineTime >= end) return doc;
  const splitSource = found.item.source_in + (timelineTime - start) * itemSpeed(found.item);
  const left: EditorItem = { ...found.item, source_out: splitSource };
  const right: EditorItem = {
    ...found.item,
    id: nextClipId(doc),
    timeline_start: timelineTime,
    source_in: splitSource,
  };
  return {
    ...doc,
    tracks: tracksOf(doc).map((track) => {
      if (track.id !== found.track.id) return track;
      const items: EditorItem[] = [];
      for (const item of track.items ?? []) {
        if (item.id !== itemId) {
          items.push(item);
          continue;
        }
        items.push(left, right);
      }
      return { ...track, items };
    }),
  };
}

export function deleteItem(doc: EditorDocument, itemId: string): EditorDocument {
  if (findItem(doc, itemId) === undefined) return doc;
  return {
    ...doc,
    tracks: tracksOf(doc).map((track) => ({
      ...track,
      items: (track.items ?? []).filter((item) => item.id !== itemId),
    })),
    transitions: (doc.transitions ?? []).filter((transition) => transition.after_item !== itemId),
  };
}

export function duplicateItem(doc: EditorDocument, itemId: string): EditorDocument {
  const found = findItem(doc, itemId);
  if (found === undefined) return doc;
  const copy: EditorItem = {
    ...found.item,
    id: nextClipId(doc),
    timeline_start: itemTimelineEnd(found.item),
  };
  return {
    ...doc,
    tracks: tracksOf(doc).map((track) => {
      if (track.id !== found.track.id) return track;
      const items: EditorItem[] = [];
      for (const item of track.items ?? []) {
        items.push(item);
        if (item.id === itemId) items.push(copy);
      }
      return { ...track, items };
    }),
  };
}

export function addTrack(doc: EditorDocument, kind: EditorTrackKind): EditorDocument {
  const tracks = tracksOf(doc);
  if (tracks.length >= EDITOR_LIMITS.maxTracks) return doc;
  return {
    ...doc,
    tracks: [...tracks, { id: nextTrackId(doc, kind), kind, items: [] }],
  };
}

export function deleteTrack(doc: EditorDocument, trackId: string): EditorDocument {
  const tracks = tracksOf(doc);
  const track = tracks.find((candidate) => candidate.id === trackId);
  if (track === undefined) return doc;
  const videoCount = tracks.filter((candidate) => candidate.kind === EDITOR_TRACK_KINDS.video).length;
  if (track.kind === EDITOR_TRACK_KINDS.video && videoCount <= 1) return doc;
  const removed = new Set((track.items ?? []).map((item) => item.id));
  return {
    ...doc,
    tracks: tracks.filter((candidate) => candidate.id !== trackId),
    transitions: (doc.transitions ?? []).filter((transition) => !removed.has(transition.after_item)),
  };
}

export function setItemProps(doc: EditorDocument, itemId: string, patch: EditorItemPatch): EditorDocument {
  if (findItem(doc, itemId) === undefined) return doc;
  return mapItem(doc, itemId, (item) => ({ ...item, ...patch }));
}

export function addOverlay(doc: EditorDocument, overlay: Omit<EditorTextOverlay, 'id'> & { id?: string }): EditorDocument {
  const overlays = [...(doc.overlays ?? [])];
  if (overlays.length >= EDITOR_LIMITS.maxOverlays) return doc;
  const used = new Set(overlays.map((entry) => entry.id));
  const id = overlay.id !== undefined && overlay.id !== '' ? overlay.id : nextPrefixedId('ov-', used);
  if (used.has(id)) return doc;
  const next: EditorTextOverlay = {
    id,
    text: overlay.text,
    position_y: overlay.position_y,
    start_seconds: overlay.start_seconds,
  };
  if (overlay.end_seconds !== undefined) next.end_seconds = overlay.end_seconds;
  if (overlay.font_size !== undefined) next.font_size = overlay.font_size;
  return { ...doc, overlays: [...overlays, next] };
}

export function updateOverlay(
  doc: EditorDocument,
  overlayId: string,
  patch: Partial<Omit<EditorTextOverlay, 'id'>>,
): EditorDocument {
  const overlays = doc.overlays ?? [];
  if (!overlays.some((overlay) => overlay.id === overlayId)) return doc;
  return {
    ...doc,
    overlays: overlays.map((overlay) => (overlay.id === overlayId ? { ...overlay, ...patch } : overlay)),
  };
}

export function deleteOverlay(doc: EditorDocument, overlayId: string): EditorDocument {
  const overlays = doc.overlays ?? [];
  if (!overlays.some((overlay) => overlay.id === overlayId)) return doc;
  return { ...doc, overlays: overlays.filter((overlay) => overlay.id !== overlayId) };
}

export function setTransitionAfter(
  doc: EditorDocument,
  itemId: string,
  kind: EditorTransitionKind,
  duration?: number,
): EditorDocument {
  if (findItem(doc, itemId) === undefined) return doc;
  const transitions = [...(doc.transitions ?? [])];
  const existing = transitions.find((transition) => transition.after_item === itemId);
  const used = new Set(transitions.map((transition) => transition.id));
  const next: EditorTransition = {
    id: existing?.id ?? nextPrefixedId('tr-', used),
    kind,
    after_item: itemId,
  };
  if (duration !== undefined) next.duration = duration;
  if (existing === undefined) return { ...doc, transitions: [...transitions, next] };
  return {
    ...doc,
    transitions: transitions.map((transition) => (transition.after_item === itemId ? next : transition)),
  };
}

export function removeTransition(doc: EditorDocument, transitionId: string): EditorDocument {
  const transitions = doc.transitions ?? [];
  if (!transitions.some((transition) => transition.id === transitionId)) return doc;
  return { ...doc, transitions: transitions.filter((transition) => transition.id !== transitionId) };
}

export function setMusic(doc: EditorDocument, key: string | undefined, volume?: number): EditorDocument {
  if (key === undefined && volume === undefined) {
    return { ...doc, music: undefined };
  }
  const music: { key?: string; volume?: number } = {};
  if (key !== undefined) music.key = key;
  if (volume !== undefined) music.volume = volume;
  return { ...doc, music };
}

export function setCanvas(doc: EditorDocument, orientation: 'portrait' | 'landscape'): EditorDocument {
  return { ...doc, canvas: { ...EDITOR_CANVAS[orientation] } };
}
