import type { FullDemoSnapshot } from '../full-demo-plan.ts';
import type { OverlayLanguage } from '../overlay-language.ts';
import type { RadarPosition, RadarSize } from '../radar-overlay.ts';

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
export type Match = {
  id: string;
  map: string;
  score: string;
  playedAt: string;
  stats: MatchStats;
  decentPlays: number;
  thumbnailUrl?: string;
  source?: 'steam' | 'upload';
  /** Display name of the clipped/target player, when known. */
  player?: string;
  /** Checksum of the demo file, shared by every job made from the same demo. */
  demoSha256?: string;
  /** Orchestrator job status when known (Partidas / constructors). */
  status?: string;
  /** Human text from the orchestrator when `status` is `failed`. */
  failureReason?: string;
  /** Multi-POV jobs only: the filmed players, anchor first. */
  povPlayers?: PlayPlayer[];
};
/** A player of a multi-POV match: the one whose POV a play uses. */
export type PlayPlayer = { steamId: string; name: string };
type PlayKind = 'clean' | 'highlight';
export type Play = {
  id: string; matchId: string; label: string; kind: PlayKind; round: number; kills: number;
  weapon?: string; thumbnailUrl?: string;
  /** Source-demo clock, only present when its tick rate and range are known. */
  startSeconds?: number; endSeconds?: number; headshots?: number;
  /** Source-demo second of the first kill, where a pre-roll is measured from. */
  firstKillSeconds?: number;
  /** Engine ids of every kill's weapon ("awp"), the side the player was on, and the enemies of a clutch. */
  weapons?: string[]; side?: PlaySide; clutchVs?: number;
  /** Death clips only: how the player died at the end of the clip. */
  death?: PlayDeath;
  /** False when the recorder cannot replay this play from the victim (bot victim, demo end). */
  victimReplay?: boolean;
  /** Multi-POV matches only: whose POV the play uses. */
  player?: PlayPlayer;
  /** From the server moments (internal/moments): 0 to 1 rank, reason codes, clutch result and win chance swing. */
  score?: number; reasons?: string[]; clutchWon?: boolean; winProbSwing?: number;
};
export type PlayDeath = { killer?: string; weapon?: string; headshot: boolean };
export type PlaySide = 'CT' | 'T';
export type RenderMode = 'clean' | 'music';
export type RenderFormat = 'short-9x16' | 'landscape-16x9';
/** Victim replay speeds (renderplan.VictimReplaySpeeds); 0.5 is the default. */
export const VICTIM_REPLAY_SPEEDS = [0.25, 0.5, 1] as const;
export type VictimReplaySpeed = (typeof VICTIM_REPLAY_SPEEDS)[number];
export const DEFAULT_VICTIM_REPLAY_SPEED: VictimReplaySpeed = 0.5;
export function isVictimReplaySpeed(value: unknown): value is VictimReplaySpeed {
  return VICTIM_REPLAY_SPEEDS.some((speed) => speed === value);
}
/** Seconds of play before each play's first kill (renderplan.PreRollChoices); 3 is the analysis cut. */
export const PRE_ROLL_CHOICES = [3, 5, 8] as const;
export type PreRollSeconds = (typeof PRE_ROLL_CHOICES)[number];
export const DEFAULT_PRE_ROLL_SECONDS: PreRollSeconds = 3;
export function isPreRollSeconds(value: unknown): value is PreRollSeconds {
  return PRE_ROLL_CHOICES.some((seconds) => seconds === value);
}
/** The HUD a 16:9 summary records (renderplan.CaptureHUDs); unset keeps the preset's. */
const CAPTURE_HUDS = ['gameplay', 'deathnotices', 'clean'] as const;
export type CaptureHud = (typeof CAPTURE_HUDS)[number];
export function isCaptureHud(value: unknown): value is CaptureHud {
  return CAPTURE_HUDS.some((hud) => hud === value);
}
type KillEffect = 'clean' | 'punch-in' | 'velocity' | 'freeze-flash' | 'shake' | 'glitch';
type TransitionStyle = 'cut' | 'flash' | 'whip' | 'dip' | 'glitch' | 'zoom-whip';
type CoverStrategy = 'generated-gameplay' | 'no-cover';
/** Max length (trimmed) for the intro/outro bookend text, enforced client-side via `maxLength`. */
export const BOOKEND_TEXT_MAX_LENGTH = 80;

export const KEYDROP_CODE_RE = /^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$/;
export const DEFAULT_KEYDROP_CODE = 'ZACKCSGO';

export const AFFILIATE_FAMILY = {
  keydrop: 'KEYDROP',
  csgoskins: 'CSGOSKINS',
} as const;

export type AffiliateFamily = (typeof AFFILIATE_FAMILY)[keyof typeof AFFILIATE_FAMILY];

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
    textClass: 'left-[16.5%] right-[13%] top-[55.5%] h-[25%]',
    // The partner art bakes its label over a gradient; the preview covers it like the render does.
    coverClass: 'bg-gradient-to-b from-[#4a1604] to-[#9a3808]',
    codePrefix: 'CODE: ',
  },
  {
    id: 'jcorko',
    label: 'Jcorko',
    subtitle: 'Cebra azul',
    preview: '/brand/keydrop/jcorko.png',
    textClass: 'left-[11%] right-[8%] top-[52%] h-[26%]',
    coverClass: 'bg-gradient-to-b from-[#06182c] to-[#1180b8]',
    codePrefix: 'CODIGO: ',
  },
] as const;

export type KeyDropStyle = (typeof KEYDROP_STYLE_CATALOG)[number]['id'];

export const CSGOSKINS_STYLE_CATALOG = [
  {
    id: 'classic',
    label: 'Classic',
    subtitle: 'CSGOSkins promo',
    preview: '/brand/csgoskins/classic.svg',
    textClass: 'left-[18%] right-[18%] top-[54%] h-[22%]',
    codePrefix: 'CODE: ',
    plate: 'csgoskins-classic.png',
  },
  {
    id: 'operator',
    label: 'Operator',
    subtitle: 'CSGOSkins táctico',
    preview: '/brand/csgoskins/operator.svg',
    textClass: 'left-[28%] right-[10%] top-[44%] h-[15%]',
    codePrefix: 'CODE: ',
    plate: 'csgoskins-operator.png',
  },
] as const;

export const AFFILIATE_FAMILY_CATALOG = [
  {
    id: AFFILIATE_FAMILY.keydrop,
    label: 'KeyDrop',
    offLabel: 'Sin KeyDrop',
    styles: KEYDROP_STYLE_CATALOG,
  },
  {
    id: AFFILIATE_FAMILY.csgoskins,
    label: 'CSGOSkins',
    offLabel: 'Sin CSGOSkins',
    styles: CSGOSKINS_STYLE_CATALOG,
  },
] as const;

export function normalizeAffiliateFamily(family: string): AffiliateFamily | '' {
  const id = family.trim().toUpperCase();
  if (id === AFFILIATE_FAMILY.keydrop || id === AFFILIATE_FAMILY.csgoskins) return id;
  return '';
}

export function effectiveAffiliateFamily(family: string, style: string): AffiliateFamily | '' {
  const normalized = normalizeAffiliateFamily(family);
  if (normalized) return normalized;
  return style.trim() ? AFFILIATE_FAMILY.keydrop : '';
}

export function stylesForFamily(family: string): readonly {
  id: string;
  label: string;
  subtitle: string;
  preview: string;
  textClass: string;
  /** Painted under the live code when the plate bakes its own label into the art. */
  coverClass?: string;
  codePrefix: string;
}[] {
  const id = effectiveAffiliateFamily(family, '') || normalizeAffiliateFamily(family);
  if (id === AFFILIATE_FAMILY.csgoskins) return CSGOSKINS_STYLE_CATALOG;
  return KEYDROP_STYLE_CATALOG;
}

export function isAffiliateStyle(family: string, style: string): boolean {
  const id = style.trim().toLowerCase();
  if (!id) return false;
  return stylesForFamily(effectiveAffiliateFamily(family, id)).some((entry) => entry.id === id);
}

export function isKeyDropStyle(value: string): value is KeyDropStyle {
  return KEYDROP_STYLE_CATALOG.some((entry) => entry.id === value);
}

/**
 * Narrows a persisted stream banner style. Affiliate families share the field,
 * so a CSGOSkins id is accepted too: the banner renderer resolves the plate by
 * family, and rejecting it here would silently blank a valid banner.
 */
export function isKeyDropBannerStyle(value: string | undefined): value is KeyDropStyle {
  if (value === undefined || value === '') return false;
  return isKeyDropStyle(value) || CSGOSKINS_STYLE_CATALOG.some((entry) => entry.id === value);
}

export function affiliateFamilyLabel(family: string, style = ''): string {
  const id = effectiveAffiliateFamily(family, style);
  return AFFILIATE_FAMILY_CATALOG.find((entry) => entry.id === id)?.label ?? '';
}

export function affiliateStyleLabel(family: string, style: string): string {
  return stylesForFamily(effectiveAffiliateFamily(family, style)).find((entry) => entry.id === style)?.label ?? style;
}

export function affiliatePlateFile(family: string, style: string): string {
  const id = effectiveAffiliateFamily(family, style);
  if (id === AFFILIATE_FAMILY.csgoskins) {
    return CSGOSKINS_STYLE_CATALOG.find((entry) => entry.id === style)?.plate ?? '';
  }
  const keyDrop = KEYDROP_STYLE_CATALOG.find((entry) => entry.id === style);
  return keyDrop ? `style-${keyDrop.id}.png` : '';
}

export function affiliateDisplayLabel(family: string, style: string, code: string): string {
  const prefix =
    stylesForFamily(effectiveAffiliateFamily(family, style)).find((entry) => entry.id === style)?.codePrefix ??
    'CODE: ';
  const body = (code.trim() || DEFAULT_KEYDROP_CODE).toUpperCase();
  return `${prefix}${body}`;
}

export const DEMO_SOURCE = {
  premier: 'premier',
  professional: 'professional',
  faceit: 'faceit',
} as const;
export type DemoSource = (typeof DEMO_SOURCE)[keyof typeof DEMO_SOURCE];

export function isDemoSource(value: unknown): value is DemoSource {
  return value === DEMO_SOURCE.premier || value === DEMO_SOURCE.professional || value === DEMO_SOURCE.faceit;
}

export const OVERLAY_THEME = {
  faceitOrange: 'faceit-orange',
  neonViolet: 'neon-violet',
  /** Full Demo only: the intro and scoreboard in the colours of the plan's HUD. */
  hud: 'hud',
} as const;
export type OverlayTheme = (typeof OVERLAY_THEME)[keyof typeof OVERLAY_THEME];

export function isOverlayTheme(value: unknown): value is OverlayTheme {
  return value === OVERLAY_THEME.faceitOrange || value === OVERLAY_THEME.neonViolet || value === OVERLAY_THEME.hud;
}

export type EditConfig = {
  fullDemo?: FullDemoSnapshot;
  format: RenderFormat;
  killEffect: KillEffect;
  transition: TransitionStyle;
  intro: boolean;
  outro: boolean;
  hookText: boolean;
  killCounter: boolean;
  /** Short only: kill notices drawn by ClipHub, centered near the top. */
  centeredKillfeed?: boolean;
  /** 9:16 only: a compact scoreboard and alive counts from the demo above the centered kill feed. */
  verticalHud?: boolean;
  /** A HUD catalog design for the vertical HUD; unset uses the first one. */
  verticalHudTheme?: string;
  /** ClipHub's radar in a corner, read from the demo; unset corner, size and opacity are the defaults. */
  radar?: boolean;
  radarPosition?: RadarPosition;
  radarSize?: RadarSize;
  radarOpacity?: number;
  overlayLanguage?: OverlayLanguage;
  matchRecap: boolean;
  /** The plays are death clips (death plan), not kills. */
  deathReel?: boolean;
  /** Auto-Shorts: each play is rendered as its own Short from one capture. */
  separateShorts?: boolean;
  voiceComms: boolean;
  voiceVolume?: number;
  /** What the team voices say, burned with the speaker's name; it needs voiceComms. */
  subtitles?: boolean;
  /** Phrase ids of the demo transcript this video leaves out. */
  subtitlesHidden?: string[];
  nativeHud: boolean;
  /** Shorts only: replay each play's last kill from the victim, at victimReplaySpeed. */
  victimReplay?: boolean;
  /** Unset plays the replay at DEFAULT_VICTIM_REPLAY_SPEED. */
  victimReplaySpeed?: VictimReplaySpeed;
  /** One player only: unset is DEFAULT_PRE_ROLL_SECONDS. */
  preRollSeconds?: PreRollSeconds;
  /** 16:9 without the native HUD: the capture profile of the summary's style. */
  captureHud?: CaptureHud;
  /** 16:9 made from a Short (Crear horizontal): CS2's kill feed where the Short's crop keeps it, so its clips are reused. */
  portraitSafeKillfeed?: boolean;
  /** A badge naming each play's clutch, ace or multikill. */
  playBadges?: boolean;
  /** Each team's round-win chance after the plant, from the kill plan. */
  winProbability?: boolean;
  /** Shorts only: name and health over teammates and over enemies the POV spotted or hit, read from the demo. */
  playerLabels?: boolean;
  /** Shorts only: the flight path of the POV team's grenades, read from the demo. */
  grenadeTrails?: boolean;
  /** 9:16 only: a closing card with the player's match stats and FACEIT data when known. */
  statsCard?: boolean;
  /** One-player 9:16 Shorts only: a player card template id per slot; the end card replaces statsCard. */
  openingCard?: string;
  endCard?: string;
  coverCard?: string;
  /** Shorts only: the brand kit's logo as a watermark; the render copies the saved kit. */
  brandWatermark?: boolean;
  /** Shorts only: the brand kit's own sponsor banner; it replaces the affiliate banner. */
  brandBanner?: boolean;
  subscribeBanner?: boolean;
  coverStrategy: CoverStrategy;
  introText?: string;
  outroText?: string;
  keyDropFamily?: AffiliateFamily | '';
  keyDropStyle?: KeyDropStyle | '';
  keyDropCode?: string;
  keyDropPositionY?: number;
  keyDropStartSeconds?: number;
  keyDropEndSeconds?: number;
  demoSource?: DemoSource;
  overlayTheme?: OverlayTheme;
};
export type Song = { id: string; title: string; artist: string; genre: string; previewUrl: string; durationSec: number; license?: string };
/** User-selectable reel preset; `name` is the render variant. */
export type Preset = {
  name: string;
  label: string;
  description: string;
  hudMode?: string;
  default?: boolean;
  width?: number;
  height?: number;
};
export type VideoStatus =
  | 'queued'
  | 'recording'
  | 'composing'
  | 'ready'
  | 'failed';
/** Live job progress during capture or editing; percent is 0-100. */
export type CaptureProgress = { done: number; total: number; percent?: number; stage?: string };
/** `GET /api/demos/{jobId}/status`: the orchestrator's `jobStatusResponse`. */
export type JobStatusView = {
  status: string;
  failureReason?: string;
  /** Stable obs class (`demo_incompatible`, `capture_flake`, …); `failureReason` stays human text. */
  failureCode?: string;
  captureProgress?: CaptureProgress;
  generateRun?: GenerateRun;
};
export type GenerateFailure = { variant: string; reason: string; code?: string; failedAt?: string };
export type GenerateRun = { variant: string; active: true } | (GenerateFailure & { active: false });
/** `jobId` links a reel back to the parsed demo it was forged from (the series view groups reels per map); absent only on mock/demo seed videos. */
export type Video = {
  id: string;
  /** Delivered render found on disk without a local reel intent: view, download and delete only. */
  recovered?: boolean;
  /** False when the video has no local job, so no publish assistant can be built for it. */
  publishable?: false;
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
  /** Stable failure class from the orchestrator job when the job itself failed. */
  failureCode?: string;
  /** Informational render QA warnings; they never block the reel. */
  warnings?: string[];
  /** Immutable render revision that produced this reel. */
  reviewArtifactPrefix?: string;
  /** Immutable revision used for this Full Demo's MP4, cover and evidence links. */
  artifactRevision?: string;
  /** Live progress during capture or editing. */
  captureProgress?: CaptureProgress;
  /** Cover candidate basenames when the render produced JPGs. */
  coverCandidates?: string[];
  /** Basename previously stored as the canonical cover, if any. */
  selectedCoverName?: string;
  /** Set only on a failed reel whose orchestrator job is gone: retry can never succeed, so the card offers delete/re-forge instead. */
  unrecoverable?: true;
  /** Ready with its last render while a stored change waits for Reintentar: applying it records with CS2, which a page load never starts. `failureReason` is set when that recording was tried and failed. */
  changePending?: true;
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

/** An existing job's status plus its roster; `players` is empty until the scan has run. */
export type ScannedDemo = {
  status: string;
  players: DemoPlayer[];
  match?: RosterMatch;
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
/** Client code when a job never reached the awaited status within the poll budget. */
export const JOB_WAIT_TIMEOUT_CODE = 'job_wait_timeout';
/** Orchestrator 503 for a feature that is not set up; never the proxy's own code. */
export const NOT_CONFIGURED_CODE = 'not_configured';
export const GENERATE_WORK_ACTIVE_CODE = 'generate_work_active';
/** 409 to a generate sent with allow_capture false whose record task would open CS2. */
export const CAPTURE_REQUIRED_CODE = 'capture_required';
export const FACEIT_NOT_CONFIGURED_CODE = 'faceit_not_configured';
export const STEAM_CODES = {
  credentialsRequired: 'steam_credentials_required',
  historyNotConfigured: 'history_not_configured',
  needKnownCode: 'need_known_code',
  demoUnavailable: 'demo_unavailable',
  accountInvalid: 'steam_account_invalid',
  invalidShareCode: 'invalid_share_code',
} as const;

/**
 * Every demo job status the orchestrator can emit, mirroring
 * `internal/job/job.go` statusNames. Any status-driven gate must derive from
 * this tuple so a new Go status cannot be silently absent on the client.
 */
export const JOB_STATUSES = [
  'queued',
  'parsing',
  'parsed',
  'recording',
  'recorded',
  'composing',
  'composed',
  'done',
  'failed',
  'scanning',
  'scanned',
  'review_required',
] as const;
export type JobStatus = (typeof JOB_STATUSES)[number];

/** Statuses at or past which the kill plan exists and stays available. */
export const PLAN_READY_STATUSES: ReadonlySet<string> = new Set<JobStatus>([
  'parsed',
  'recording',
  'recorded',
  'composing',
  'composed',
  'review_required',
  'done',
]);

/** Imported demos may parse a configured player before a roster exists. */
/** Terminal: the demo could not be scanned or parsed; it lists with its reason so it can be cleared. */
export const MATCH_STATUS_FAILED = 'failed';

export const SCAN_PENDING_STATUSES: ReadonlySet<string> = new Set<JobStatus>(['queued', 'scanning', 'parsing']);

/** Statuses with a roster scan, so the demo belongs in Partidas: scanned or anything after it. */
export const ROSTER_READY_STATUSES: ReadonlySet<string> = new Set<JobStatus>(['scanned', 'parsing', ...(PLAN_READY_STATUSES as ReadonlySet<JobStatus>)]);

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
