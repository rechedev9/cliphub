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
/** KeyDrop plate style for demo reels; empty/undefined disables the banner. */
export type KeyDropStyle = 'operator' | 'classic';

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
/** Live capture progress (segments done/total); set only while status is 'recording'. */
export type CaptureProgress = { done: number; total: number };
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
  /** Cover candidate basenames; the Library thumbnail gate picks among these. */
  coverCandidates?: string[];
  /** Basename the user approved as the canonical cover; unset until the second gate. */
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
