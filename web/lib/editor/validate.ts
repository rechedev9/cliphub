import {
  EDITOR_CANVAS,
  EDITOR_FILTERS,
  EDITOR_SCHEMA_VERSION,
  EDITOR_TRACK_KINDS,
  EDITOR_TRANSITIONS,
  documentDuration,
  itemOutputDuration,
  type EditorDocument,
  type EditorItem,
  type EditorTextOverlay,
  type EditorTrack,
  type EditorTransform,
  type EditorTransition,
} from './evaluate.ts';

export const EDITOR_LIMITS = {
  maxTracks: 8,
  maxItemsPerTrack: 64,
  maxOverlays: 8,
  minSpeed: 0.25,
  maxSpeed: 3,
  maxVolume: 2,
  maxFadeSeconds: 5,
  maxTextRunes: 120,
  minFontSize: 24,
  maxFontSize: 120,
  minVerticalCenterY: 0.025,
  maxVerticalCenterY: 0.975,
  musicVolumeMax: 1,
} as const;

const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;
const ASSET_ID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const MUSIC_KEY_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

const PORTRAIT = EDITOR_CANVAS.portrait;
const LANDSCAPE = EDITOR_CANVAS.landscape;

function finiteNonNeg(value: number): boolean {
  return Number.isFinite(value) && value >= 0;
}

function finitePositive(value: number): boolean {
  return Number.isFinite(value) && value > 0;
}

function finiteUnit(value: number): boolean {
  return Number.isFinite(value) && value >= 0 && value <= 1;
}

function validateCanvas(canvas: EditorDocument['canvas']): string | undefined {
  const portrait = canvas.width === PORTRAIT.width && canvas.height === PORTRAIT.height;
  const landscape = canvas.width === LANDSCAPE.width && canvas.height === LANDSCAPE.height;
  if (!portrait && !landscape) {
    return `canvas must be ${PORTRAIT.width}x${PORTRAIT.height} or ${LANDSCAPE.width}x${LANDSCAPE.height}`;
  }
  if (canvas.fps !== PORTRAIT.fps) {
    return `canvas fps must be ${PORTRAIT.fps}`;
  }
  return undefined;
}

function validateTransform(transform: EditorTransform, itemId: string): string | undefined {
  if (!finiteUnit(transform.x) || !finiteUnit(transform.y) || !finitePositive(transform.width) || !finitePositive(transform.height)) {
    return `item ${itemId} transform must use finite normalized coordinates`;
  }
  if (transform.x + transform.width > 1 || transform.y + transform.height > 1) {
    return `item ${itemId} transform must stay within the canvas`;
  }
  const opacity = transform.opacity ?? 0;
  if (opacity !== 0 && (Number.isNaN(opacity) || opacity < 0 || opacity > 1)) {
    return `item ${itemId} opacity must be between 0 and 1`;
  }
  return undefined;
}

function validateItem(item: EditorItem, trackId: string): string | undefined {
  if (!ID_PATTERN.test(item.id)) {
    return `invalid item id ${JSON.stringify(item.id)}`;
  }
  if (!ASSET_ID_PATTERN.test(item.asset_id)) {
    return `item ${item.id} asset_id must be a uuid`;
  }
  if (!finiteNonNeg(item.timeline_start)) {
    return `item ${item.id} timeline_start must be finite and >= 0`;
  }
  if (!finiteNonNeg(item.source_in)) {
    return `item ${item.id} source_in must be finite and >= 0`;
  }
  if (!Number.isFinite(item.source_out) || item.source_out <= item.source_in) {
    return `item ${item.id} source_out must be greater than source_in`;
  }
  const speed = item.speed ?? 0;
  if (speed !== 0 && (Number.isNaN(speed) || speed < EDITOR_LIMITS.minSpeed || speed > EDITOR_LIMITS.maxSpeed)) {
    return `item ${item.id} speed must be between 0.25 and 3`;
  }
  if (item.volume !== undefined && (Number.isNaN(item.volume) || item.volume < 0 || item.volume > EDITOR_LIMITS.maxVolume)) {
    return `item ${item.id} volume must be between 0 and 2`;
  }
  const fadeIn = item.fade_in ?? 0;
  if (Number.isNaN(fadeIn) || fadeIn < 0 || fadeIn > EDITOR_LIMITS.maxFadeSeconds) {
    return `item ${item.id} fade_in must be between 0 and 5`;
  }
  const fadeOut = item.fade_out ?? 0;
  if (Number.isNaN(fadeOut) || fadeOut < 0 || fadeOut > EDITOR_LIMITS.maxFadeSeconds) {
    return `item ${item.id} fade_out must be between 0 and 5`;
  }
  if (fadeIn + fadeOut > itemOutputDuration({ ...item, fade_in: fadeIn, fade_out: fadeOut })) {
    return `item ${item.id} fades must fit within the item output duration`;
  }
  const filter = item.filter ?? EDITOR_FILTERS.none;
  if (filter !== EDITOR_FILTERS.none && filter !== EDITOR_FILTERS.grade) {
    return `item ${item.id} filter must be empty or grade`;
  }
  if (item.transform !== undefined) {
    const transformError = validateTransform(item.transform, item.id);
    if (transformError !== undefined) return transformError;
  }
  if (trackId === '') {
    return `item ${item.id} is missing its track`;
  }
  return undefined;
}

function validateTrack(track: EditorTrack): string | undefined {
  if (!ID_PATTERN.test(track.id)) {
    return `invalid track id ${JSON.stringify(track.id)}`;
  }
  if (track.kind !== EDITOR_TRACK_KINDS.video && track.kind !== EDITOR_TRACK_KINDS.audio) {
    return `track ${track.id} kind must be video or audio`;
  }
  const items = track.items ?? [];
  if (items.length > EDITOR_LIMITS.maxItemsPerTrack) {
    return `track ${track.id} has at most ${EDITOR_LIMITS.maxItemsPerTrack} items`;
  }
  for (const item of items) {
    const itemError = validateItem(item, track.id);
    if (itemError !== undefined) return itemError;
  }
  return undefined;
}

function validateOverlay(overlay: EditorTextOverlay, timelineDuration: number): string | undefined {
  if (!ID_PATTERN.test(overlay.id)) {
    return `invalid overlay id ${JSON.stringify(overlay.id)}`;
  }
  const text = overlay.text.trim();
  if (text === '') {
    return `overlay ${overlay.id} text is required`;
  }
  if ([...text].length > EDITOR_LIMITS.maxTextRunes) {
    return `overlay ${overlay.id} text must be at most ${EDITOR_LIMITS.maxTextRunes} characters`;
  }
  for (const rune of text) {
    const code = rune.codePointAt(0) ?? 0;
    if (code < 0x20 || code === 0x7f) {
      return `overlay ${overlay.id} text must not contain control characters`;
    }
  }
  if (
    Number.isNaN(overlay.position_y)
    || !Number.isFinite(overlay.position_y)
    || overlay.position_y < EDITOR_LIMITS.minVerticalCenterY
    || overlay.position_y > EDITOR_LIMITS.maxVerticalCenterY
  ) {
    return `overlay ${overlay.id} position_y must be between 0.025 and 0.975`;
  }
  const fontSize = overlay.font_size ?? 0;
  if (fontSize !== 0 && (fontSize < EDITOR_LIMITS.minFontSize || fontSize > EDITOR_LIMITS.maxFontSize)) {
    return `overlay ${overlay.id} font_size must be between 24 and 120`;
  }
  if (!finiteNonNeg(overlay.start_seconds)) {
    return `overlay ${overlay.id} start_seconds must be finite and >= 0`;
  }
  let end = timelineDuration;
  if (overlay.end_seconds !== undefined) {
    end = overlay.end_seconds;
    if (!Number.isFinite(end) || end <= overlay.start_seconds) {
      return `overlay ${overlay.id} end_seconds must be greater than start_seconds`;
    }
  }
  if (timelineDuration > 0 && overlay.start_seconds >= timelineDuration) {
    return `overlay ${overlay.id} start_seconds must be inside the timeline`;
  }
  if (timelineDuration > 0 && end > timelineDuration + 0.001) {
    return `overlay ${overlay.id} end_seconds exceeds timeline duration`;
  }
  return undefined;
}

function validateTransition(transition: EditorTransition, items: Map<string, EditorItem>): string | undefined {
  if (!ID_PATTERN.test(transition.id)) {
    return `invalid transition id ${JSON.stringify(transition.id)}`;
  }
  if (transition.kind !== EDITOR_TRANSITIONS.cut && transition.kind !== EDITOR_TRANSITIONS.crossfade) {
    return `transition ${transition.id} kind must be cut or crossfade`;
  }
  if (!items.has(transition.after_item)) {
    return `transition ${transition.id} after_item ${JSON.stringify(transition.after_item)} is unknown`;
  }
  if (transition.kind === EDITOR_TRANSITIONS.crossfade) {
    const duration = transition.duration ?? Number.NaN;
    if (Number.isNaN(duration) || duration <= 0 || duration > EDITOR_LIMITS.maxFadeSeconds) {
      return `transition ${transition.id} duration must be in (0, 5]`;
    }
  }
  return undefined;
}

function firstValidateError(doc: EditorDocument): string | undefined {
  if (doc.schema_version !== '' && doc.schema_version !== EDITOR_SCHEMA_VERSION) {
    return `schema_version must be ${JSON.stringify(EDITOR_SCHEMA_VERSION)}`;
  }
  const canvasError = validateCanvas(doc.canvas);
  if (canvasError !== undefined) return canvasError;
  const tracks = doc.tracks ?? [];
  if (tracks.length === 0) {
    return 'timeline needs at least one track';
  }
  if (tracks.length > EDITOR_LIMITS.maxTracks) {
    return `timeline has at most ${EDITOR_LIMITS.maxTracks} tracks`;
  }
  const seenTracks = new Set<string>();
  const seenItems = new Map<string, EditorItem>();
  let hasVideo = false;
  for (const track of tracks) {
    const trackError = validateTrack(track);
    if (trackError !== undefined) return trackError;
    if (seenTracks.has(track.id)) {
      return `duplicate track id ${JSON.stringify(track.id)}`;
    }
    seenTracks.add(track.id);
    if (track.kind === EDITOR_TRACK_KINDS.video) hasVideo = true;
    for (const item of track.items ?? []) {
      if (seenItems.has(item.id)) {
        return `duplicate item id ${JSON.stringify(item.id)}`;
      }
      seenItems.set(item.id, item);
    }
  }
  if (!hasVideo) {
    return 'timeline needs at least one video track';
  }
  const overlays = doc.overlays ?? [];
  if (overlays.length > EDITOR_LIMITS.maxOverlays) {
    return `timeline has at most ${EDITOR_LIMITS.maxOverlays} text overlays`;
  }
  const duration = documentDuration(doc);
  const seenOverlays = new Set<string>();
  for (const overlay of overlays) {
    const overlayError = validateOverlay(overlay, duration);
    if (overlayError !== undefined) return overlayError;
    if (seenOverlays.has(overlay.id)) {
      return `duplicate overlay id ${JSON.stringify(overlay.id)}`;
    }
    seenOverlays.add(overlay.id);
  }
  const seenTransitions = new Set<string>();
  for (const transition of doc.transitions ?? []) {
    const transitionError = validateTransition(transition, seenItems);
    if (transitionError !== undefined) return transitionError;
    if (seenTransitions.has(transition.id)) {
      return `duplicate transition id ${JSON.stringify(transition.id)}`;
    }
    seenTransitions.add(transition.id);
  }
  const musicKey = doc.music?.key ?? '';
  if (musicKey !== '' && !MUSIC_KEY_PATTERN.test(musicKey)) {
    return `invalid music key ${JSON.stringify(musicKey)}`;
  }
  const musicVolume = doc.music?.volume ?? 0;
  if (musicVolume < 0 || musicVolume > EDITOR_LIMITS.musicVolumeMax) {
    return 'music volume must be between 0 and 1';
  }
  return undefined;
}

function wrap(error: string | undefined): string[] {
  return error === undefined ? [] : [error];
}

export function validateDocument(doc: EditorDocument): string[] {
  return wrap(firstValidateError(doc));
}

export function validateForRender(doc: EditorDocument): string[] {
  const error = firstValidateError(doc);
  if (error !== undefined) return [error];
  let items = 0;
  for (const track of doc.tracks ?? []) {
    items += (track.items ?? []).length;
  }
  if (items === 0) return ['timeline has no items'];
  return [];
}
