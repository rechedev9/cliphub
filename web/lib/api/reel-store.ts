import type { EditConfig, RenderMode } from './types';

// Mirrors types.BOOKEND_TEXT_MAX_LENGTH; duplicated so Node's TS loader stays type-only.
const BOOKEND_TEXT_MAX_LENGTH = 80;

/** Persisted reel request. Live status comes from the orchestrator, not storage. */
export type ReelIntent = {
  videoId: string; // `${jobId}__${segmentIds.join('_')}`
  jobId: string;
  /** Segment ids in plan order; 2+ ids render as one concatenated reel. */
  segmentIds: string[];
  mode: RenderMode;
  /** Render variant / preset name (Kill Feed / Clean POV / Full HUD). */
  variant?: string;
  editConfig: EditConfig;
  songId?: string;
  /** Music gain in (0,1]; absent means full volume (1.0). */
  musicVolume?: number;
  /** Game-audio gain in [0,1] when music is mixed; absent keeps the 0.20 duck. */
  gameVolume?: number;
  title: string;
  map: string;
  score: string;
  /** Display name for the selected SteamID; optional for migrated intents. */
  targetName?: string;
  /** Approved cover basename; absent means the thumbnail gate is still open. */
  selectedCoverName?: string;
  createdAt: number;
};

const STORE_KEY = 'cliphub.reels.v1';
/** Pre-rebrand key; read once and migrate into STORE_KEY. */
const LEGACY_STORE_KEY = 'fragforge.reels.v1';
/** Keep localStorage bounded; newest intents win. */
const MAX_INTENTS = 50;

/** Default variant and migration target for pre-preset intents. */
export const DEFAULT_VARIANT = 'viral-60-clean';

export const DEFAULT_EDIT_CONFIG: EditConfig = {
  format: 'short-9x16',
  killEffect: 'punch-in',
  transition: 'flash',
  intro: false,
  outro: false,
  hookText: false,
  killCounter: false,
  matchRecap: false,
  voiceComms: false,
  nativeHud: false,
  coverStrategy: 'generated-gameplay',
  introText: '',
  outroText: '',
};

export function loadReelIntents(): ReelIntent[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = window.localStorage.getItem(STORE_KEY);
    if (raw) return coerceIntents(JSON.parse(raw));
    // One-shot migration from the pre-rebrand localStorage key.
    const legacy = window.localStorage.getItem(LEGACY_STORE_KEY);
    if (!legacy) return [];
    const intents = coerceIntents(JSON.parse(legacy));
    if (intents.length > 0) {
      saveReelIntents(intents);
      window.localStorage.removeItem(LEGACY_STORE_KEY);
    }
    return intents;
  } catch {
    return []; // corrupt / unavailable storage: reels are best-effort.
  }
}

export function saveReelIntents(list: ReelIntent[]): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORE_KEY, JSON.stringify(list.slice(-MAX_INTENTS)));
  } catch {
    // quota / privacy mode: in-memory reels still work this session.
  }
}

/** Drop malformed parsed JSON; default soft fields. Pure and unit-tested. */
export function coerceIntents(parsed: unknown): ReelIntent[] {
  if (!Array.isArray(parsed)) return [];
  const out: ReelIntent[] = [];
  for (const v of parsed) {
    if (!v || typeof v !== 'object') continue;
    const r = v as Record<string, unknown>;
    if (typeof r.videoId !== 'string' || typeof r.jobId !== 'string') continue;
    const segmentIds = coerceSegmentIds(r);
    if (segmentIds.length === 0) continue;
    const intent: ReelIntent = {
      videoId: r.videoId,
      jobId: r.jobId,
      segmentIds,
      mode: r.mode === 'music' ? 'music' : 'clean',
      variant: typeof r.variant === 'string' ? r.variant : DEFAULT_VARIANT,
      editConfig: coerceEditConfig(r.editConfig),
      songId: typeof r.songId === 'string' ? r.songId : undefined,
      musicVolume: coerceMusicVolume(r.musicVolume),
      title: typeof r.title === 'string' ? r.title : 'Highlight',
      map: typeof r.map === 'string' ? r.map : 'Unknown',
      score: typeof r.score === 'string' ? r.score : '',
      targetName: typeof r.targetName === 'string' && r.targetName.trim() !== '' ? r.targetName.trim() : undefined,
      createdAt: typeof r.createdAt === 'number' ? r.createdAt : 0,
    };
    const gameVolume = coerceUnitVolume(r.gameVolume);
    if (gameVolume !== undefined) intent.gameVolume = gameVolume;
    if (typeof r.selectedCoverName === 'string' && r.selectedCoverName.trim() !== '') {
      intent.selectedCoverName = r.selectedCoverName.trim();
    }
    out.push(intent);
  }
  return out;
}

/** Current segmentIds, or a legacy singular segmentId. Drops non-strings. */
function coerceSegmentIds(r: Record<string, unknown>): string[] {
  if (Array.isArray(r.segmentIds)) {
    return r.segmentIds.filter((entry): entry is string => typeof entry === 'string' && entry.length > 0);
  }
  if (typeof r.segmentId === 'string' && r.segmentId.length > 0) return [r.segmentId];
  return [];
}

export function coerceEditConfig(value: unknown): EditConfig {
  if (!value || typeof value !== 'object') return DEFAULT_EDIT_CONFIG;
  const raw = value as Partial<EditConfig>;
  const cfg: EditConfig = {
    format: raw.format === 'landscape-16x9' ? 'landscape-16x9' : DEFAULT_EDIT_CONFIG.format,
    killEffect: isKillEffect(raw.killEffect) ? raw.killEffect : DEFAULT_EDIT_CONFIG.killEffect,
    transition: isTransition(raw.transition) ? raw.transition : DEFAULT_EDIT_CONFIG.transition,
    intro: raw.intro === true,
    outro: raw.outro === true,
    hookText: raw.hookText === true,
    killCounter: raw.killCounter === true,
    matchRecap: raw.matchRecap === true,
    voiceComms: raw.voiceComms === true,
    nativeHud: raw.nativeHud === true,
    coverStrategy: raw.coverStrategy === 'no-cover' ? 'no-cover' : DEFAULT_EDIT_CONFIG.coverStrategy,
    introText: coerceBookendText(raw.introText),
    outroText: coerceBookendText(raw.outroText),
  };
  const voiceVolume = coerceUnitVolume(raw.voiceVolume);
  if (voiceVolume !== undefined) cfg.voiceVolume = voiceVolume;
  if (raw.keyDropStyle === 'operator' || raw.keyDropStyle === 'classic') {
    cfg.keyDropStyle = raw.keyDropStyle;
  }
  if (typeof raw.keyDropCode === 'string' && raw.keyDropCode.trim() !== '') {
    cfg.keyDropCode = raw.keyDropCode.trim().toUpperCase().slice(0, 16);
  }
  if (
    typeof raw.keyDropPositionY === 'number' &&
    raw.keyDropPositionY >= 0.025 &&
    raw.keyDropPositionY <= 0.975
  ) {
    cfg.keyDropPositionY = raw.keyDropPositionY;
  }
  if (typeof raw.keyDropStartSeconds === 'number' && raw.keyDropStartSeconds >= 0) {
    cfg.keyDropStartSeconds = raw.keyDropStartSeconds;
  }
  if (typeof raw.keyDropEndSeconds === 'number' && raw.keyDropEndSeconds > 0) {
    cfg.keyDropEndSeconds = raw.keyDropEndSeconds;
  }
  return cfg;
}

/** Accept only a real number in (0,1]; anything else becomes undefined. */
function coerceMusicVolume(value: unknown): number | undefined {
  return typeof value === 'number' && value > 0 && value <= 1 ? value : undefined;
}

function coerceUnitVolume(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 1 ? value : undefined;
}

function coerceBookendText(value: unknown): string {
  return typeof value === 'string' ? value.slice(0, BOOKEND_TEXT_MAX_LENGTH) : '';
}

function isKillEffect(value: unknown): value is EditConfig['killEffect'] {
  return (
    value === 'clean' ||
    value === 'punch-in' ||
    value === 'velocity' ||
    value === 'freeze-flash' ||
    value === 'shake' ||
    value === 'glitch'
  );
}

function isTransition(value: unknown): value is EditConfig['transition'] {
  return (
    value === 'cut' ||
    value === 'flash' ||
    value === 'whip' ||
    value === 'dip' ||
    value === 'glitch' ||
    value === 'zoom-whip'
  );
}
