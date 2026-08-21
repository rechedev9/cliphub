export type SteamUser = { id: string; personaName: string; avatarUrl: string };
export type MatchStats = {
  kills: number;
  deaths: number;
  assists: number;
  mvps: number;
  kd: number;
  rating?: number;
  adr?: number;
  kast?: number;
  hsPct?: number;
};
export type Match = { id: string; map: string; score: string; playedAt: string; stats: MatchStats; decentPlays: number; thumbnailUrl?: string; source?: 'steam' | 'upload'; /** Display name of the clipped/target player, when known. */ player?: string };
export type PlayKind = 'clean' | 'highlight';
export type Play = { id: string; matchId: string; label: string; kind: PlayKind; round: number; kills: number; weapon?: string; thumbnailUrl?: string };
export type RenderMode = 'clean' | 'music';
export type RenderFormat = 'short-9x16' | 'landscape-16x9';
export type KillEffect = 'clean' | 'punch-in' | 'velocity' | 'freeze-flash' | 'shake' | 'glitch';
export type TransitionStyle = 'cut' | 'flash' | 'whip' | 'dip' | 'glitch' | 'zoom-whip';
export type CoverStrategy = 'generated-gameplay' | 'no-cover';
/** Max length (trimmed) for the intro/outro bookend text, enforced client-side via `maxLength`. */
export const BOOKEND_TEXT_MAX_LENGTH = 80;

export const KEYDROP_CODE_RE = /^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$/;
export const DEFAULT_KEYDROP_CODE = 'ZACKCSGO';

export const KEYDROP_STYLE_CATALOG = [
  {
    id: 'operator',
    label: 'Operator',
    subtitle: 'KeyDrop táctico',
    preview: '/brand/keydrop/operator.png',
    textClass: 'left-[28%] right-[10%] top-[44%] h-[15%]',
    codePrefix: 'CODE: ',
  },
  {
    id: 'classic',
    label: 'Classic',
    subtitle: 'Promo con regalo',
    preview: '/brand/keydrop/classic.png',
    textClass: 'left-[18%] right-[18%] top-[54%] h-[22%]',
    codePrefix: 'CODE: ',
  },
  {
    id: 'tigerr',
    label: 'Tigerr',
    subtitle: 'Tiger Tooth naranja',
    preview: '/brand/keydrop/tigerr.png',
    textClass: 'left-[20%] right-[20%] top-[48%] h-[24%]',
    codePrefix: 'CODE: ',
  },
  {
    id: 'jcorko',
    label: 'Jcorko',
    subtitle: 'Cebra azul',
    preview: '/brand/keydrop/jcorko.png',
    textClass: 'left-[20%] right-[20%] top-[48%] h-[24%]',
    codePrefix: 'CODIGO: ',
  },
] as const;

export type KeyDropStyle = (typeof KEYDROP_STYLE_CATALOG)[number]['id'];

export function isKeyDropStyle(value: string): value is KeyDropStyle {
  return KEYDROP_STYLE_CATALOG.some((entry) => entry.id === value);
}

export function keyDropStyleLabel(id: string): string {
  return KEYDROP_STYLE_CATALOG.find((entry) => entry.id === id)?.label ?? id;
}

export function keyDropDisplayLabel(style: KeyDropStyle | '', code: string): string {
  const prefix = KEYDROP_STYLE_CATALOG.find((entry) => entry.id === style)?.codePrefix ?? 'CODE: ';
  const body = (code.trim() || DEFAULT_KEYDROP_CODE).toUpperCase();
  return `${prefix}${body}`;
}

export type EditConfig = {
  format: RenderFormat;
  killEffect: KillEffect;
  transition: TransitionStyle;
  intro: boolean;
  outro: boolean;
  hookText: boolean;
  killCounter: boolean;
  matchRecap: boolean;
  voiceComms: boolean;
  voiceVolume?: number;
  nativeHud: boolean;
  coverStrategy: CoverStrategy;
  introText?: string;
  outroText?: string;
  keyDropStyle?: KeyDropStyle | '';
  keyDropCode?: string;
  keyDropPositionY?: number;
  keyDropStartSeconds?: number;
  keyDropEndSeconds?: number;
};
export type Song = { id: string; title: string; artist: string; genre: string; previewUrl: string; durationSec: number; license?: string };
/** User-selectable reel preset; `name` is the render variant. */
export type Preset = { name: string; label: string; description: string; hudMode?: string; default?: boolean };
export type VideoStatus =
  | 'queued'
  | 'recording'
  | 'composing'
  | 'ready'
  | 'review_required'
  | 'failed';
/** Live capture progress; set only while status is 'recording'. percent is 0-100 of planned ticks. */
export type CaptureProgress = { done: number; total: number; percent?: number };
/** `jobId` links a reel back to the parsed demo it was forged from (the series view groups reels per map); absent only on mock/demo seed videos. */
export type Video = {
  id: string;
  jobId?: string;
  title: string;
  map: string;
  score: string;
  /** Display name of the canonical SteamID target, when known. */
  targetName?: string;
  mode: RenderMode;
  variant?: string;
  songId?: string;
  musicVolume?: number;
  gameVolume?: number;
  editConfig?: EditConfig;
  status: VideoStatus;
  createdAt: number;
  availableForSec?: number;
  thumbnailUrl?: string;
  downloadUrl?: string;
  failureReason?: string;
  /** Exact render QA warnings; populated only while status is review_required. */
  warnings?: string[];
  /** Immutable artifact revision shown with `warnings`; both values form the review CAS token. */
  reviewArtifactPrefix?: string;
  captureProgress?: CaptureProgress;
  /** Cover candidate basenames when the render produced JPGs. */
  coverCandidates?: string[];
  /** Basename previously stored as the canonical cover, if any. */
  selectedCoverName?: string;
  /** Set only on a failed reel whose orchestrator job is gone: retry can never succeed, so the card offers delete/re-forge instead. */
  unrecoverable?: true;
};
export type Slots = { used: number; total: number };
export type FeedItem = { id: string; author: string; authorAvatarUrl: string; title: string; map: string; thumbnailUrl: string; likes: number; createdAt: number; videoUrl: string };
export type Session = { user: SteamUser | null; slots: Slots; pcPaired: boolean; matchHistoryLinked: boolean };
/** Roster-scan player; scoreboard extras default to 0 on older artifacts. */
export type DemoPlayer = {
  steamId: string;
  name: string;
  team: 'CT' | 'T' | '';
  kills: number;
  deaths: number;
  assists: number;
  headshots: number;
  mvps: number;
  rounds: number;
  adr: number;
  hsPct: number;
  kast: number;
  rating: number;
  /** Multi-kill round counts; absent on older artifacts, treat as 0. */
  rounds2k?: number;
  rounds3k?: number;
  rounds4k?: number;
  rounds5k?: number;
};

/** Roster header context; optional on older or incomplete scans. */
export type RosterMatch = {
  map: string;
  scoreCt: number;
  scoreT: number;
  rounds: number;
};

/** One demo of a bulk series; `match` is filled once the roster scan exists. */
export type SeriesDemo = {
  jobId: string;
  fileName?: string;
  status: string;
  failureReason?: string;
  match?: RosterMatch;
};

/** Series scoreboard: counted stats summed, rates round-weighted. */
export type AggregatedSeriesPlayer = DemoPlayer & { mapsPresent: number };

/** Proxy code when the local orchestrator is unreachable. */
export const SERVICE_UNAVAILABLE_CODE = 'service_unavailable';
export const FACEIT_NOT_CONFIGURED_CODE = 'faceit_not_configured';
export const STEAM_CODES = {
  credentialsRequired: 'steam_credentials_required',
  historyNotConfigured: 'history_not_configured',
  needKnownCode: 'need_known_code',
  demoUnavailable: 'demo_unavailable',
  accountInvalid: 'steam_account_invalid',
  invalidShareCode: 'invalid_share_code',
} as const;

/** Statuses at or past which the kill plan exists and stays available. */
export const PLAN_READY_STATUSES: ReadonlySet<string> = new Set([
  'parsed',
  'recording',
  'recorded',
  'composing',
  'composed',
  'done',
]);

/** One capture tool and how its path was resolved. */
export type CaptureTool = {
  name: string;
  path?: string;
  source?: 'env' | 'detected' | 'none';
  configured: boolean;
  accessible: boolean;
};

/** Gameplay-capture readiness on this machine. */
export type CaptureStatus = 'ready' | 'warning' | 'unconfigured' | 'offline';
export type CaptureReadiness = {
  recordEnabled: boolean;
  status: CaptureStatus;
  tools: CaptureTool[];
  reason?: string;
};
