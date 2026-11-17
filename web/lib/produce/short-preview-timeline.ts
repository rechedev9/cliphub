import type { EditConfig } from '../api/types.ts';
import { victimReplaySeconds } from './short-selection.ts';

/** The clip on screen: two plays of the example, and the victim's view of the first one's last kill. */
export type PreviewClip = 'a' | 'replay' | 'b';

export type PreviewFlash = { color: string; opacity: number };

/** Everything the preview draws at one moment, in the output frame's pixels. */
export type PreviewFrame = {
  clip: PreviewClip;
  zoom: number;
  shakeX: number;
  shakeY: number;
  /** Chroma shift in pixels. */
  glitch: number;
  grade: boolean;
  flash: PreviewFlash | null;
  counter: { value: string; size: number; opacity: number } | null;
  badge: { label: string; opacity: number } | null;
  /** Kill notices on screen, newest last. */
  notices: number;
  hook: number;
  intro: number;
  outro: number;
  stats: number;
  winProbability: number;
  verticalHud: boolean;
  phase: string;
};

export type PreviewTimeline = {
  duration: number;
  /** Absolute kill times, in Short order: two in the first play, one in the second. */
  kills: readonly number[];
  replay: { start: number; end: number } | null;
  /** Where the second play starts: the cut the transition dresses. */
  cut: number;
  statsStart: number | null;
  edit: EditConfig;
};

const CLIP_A_KILLS = [1.0, 1.9] as const;
const CLIP_A_SECONDS = 2.8;
const CLIP_B_KILL = 1.0;
const PLAY_BADGE_SECONDS = 1.8;
const STATS_CARD_SECONDS = 3.0;
const STATS_AFTER_KILL = 0.6;
const OUTRO_SECONDS = 1.6;
const NOTICE_SECONDS = 2.8;
/** A long replay would make the loop drag; the speed still shows in how slowly it plays. */
const MAX_REPLAY_SECONDS = 3;
const FLASH_HALF_SECONDS = 0.1;
/** How long the opening card and the automatic title stay on screen. */
const INTRO_SECONDS = 1.3;
const HOOK_SECONDS = 1.8;

const EFFECT_NAME: Record<EditConfig['killEffect'], string> = {
  clean: 'Limpio', 'punch-in': 'Impacto', velocity: 'Velocidad', 'freeze-flash': 'Congelado', shake: 'Terremoto', glitch: 'Glitch',
};

const TRANSITION_NAME: Record<EditConfig['transition'], string> = {
  cut: 'Corte', flash: 'Destello', whip: 'Barrido', dip: 'Fundido', glitch: 'Glitch', 'zoom-whip': 'Zoom-whip',
};

export function portraitPreview(edit: EditConfig): boolean {
  return edit.format === 'short-9x16';
}

function showsStatsCard(edit: EditConfig): boolean {
  return portraitPreview(edit) && edit.statsCard === true;
}

/** The example Short: play one, its replay when asked, play two and the closing seconds. */
export function previewTimeline(edit: EditConfig): PreviewTimeline {
  const replaySeconds = Math.min(victimReplaySeconds(edit), MAX_REPLAY_SECONDS);
  const replay = replaySeconds > 0 ? { start: CLIP_A_SECONDS, end: CLIP_A_SECONDS + replaySeconds } : null;
  const cut = replay?.end ?? CLIP_A_SECONDS;
  const lastKill = cut + CLIP_B_KILL;
  const afterKill = edit.playBadges ? PLAY_BADGE_SECONDS + 0.1 : STATS_AFTER_KILL;
  const tail = showsStatsCard(edit) ? afterKill + STATS_CARD_SECONDS : Math.max(afterKill, OUTRO_SECONDS + 0.4);
  const duration = lastKill + tail;
  return {
    duration,
    kills: [...CLIP_A_KILLS, lastKill],
    replay,
    cut,
    statsStart: showsStatsCard(edit) ? duration - STATS_CARD_SECONDS : null,
    edit,
  };
}

type Span = readonly [start: number, end: number];

function within(t: number, [start, end]: Span): boolean {
  return t >= start && t < end;
}

/** Up to the peak and back, eased, as the render's zoom crop does around the kill. */
function envelope(t: number, { start, peak, end }: { start: number; peak: number; end: number }): number {
  if (!within(t, [start, end])) return 0;
  const x = t < peak ? (t - start) / Math.max(peak - start, 0.001) : (end - t) / Math.max(end - peak, 0.001);
  return x * x * (3 - 2 * x);
}

function fade(t: number, window: { start: number; end: number; fadeIn?: number; fadeOut?: number }): number {
  if (!within(t, [window.start, window.end])) return 0;
  const rise = window.fadeIn ? Math.min(1, (t - window.start) / window.fadeIn) : 1;
  const fall = window.fadeOut ? Math.min(1, (window.end - t) / window.fadeOut) : 1;
  return Math.max(0, Math.min(rise, fall));
}

type Motion = { zoom: number; shakeX: number; shakeY: number; glitch: number; grade: boolean; flash: PreviewFlash | null };

/** The kill effects of internal/editor generatedKillEffects, with the same windows and strengths. */
function killMotion({ effect, t, kill: k }: { effect: EditConfig['killEffect']; t: number; kill: number }): Motion {
  const motion: Motion = { zoom: 1, shakeX: 0, shakeY: 0, glitch: 0, grade: false, flash: null };
  const zoom = ([before, after]: Span, scale: number): void => {
    motion.zoom = 1 + (scale - 1) * envelope(t, { start: k - before, peak: k, end: k + after });
  };
  if (effect === 'punch-in') zoom([0.16, 0.42], 1.08);
  if (effect === 'velocity') {
    zoom([0.22, 0.5], 1.16);
    motion.grade = within(t, [k - 0.1, k + 0.34]);
  }
  if (effect === 'freeze-flash') {
    zoom([0.12, 0.34], 1.1);
    if (within(t, [k - 0.02, k + 0.1])) motion.flash = { color: 'white', opacity: 0.34 };
  }
  if (effect === 'shake') {
    zoom([0.16, 0.42], 1.15);
    if (within(t, [k - 0.02, k + 0.38])) {
      const phase = 2 * Math.PI * 18 * (t - k);
      motion.shakeX = 16 * Math.sin(phase);
      motion.shakeY = 16 * Math.cos(phase * 1.3);
    }
  }
  if (effect === 'glitch') {
    zoom([0.1, 0.28], 1.12);
    if (within(t, [k - 0.02, k + 0.14])) motion.glitch = 12;
    if (within(t, [k - 0.01, k + 0.06])) motion.flash = { color: '#00ffff', opacity: 0.22 };
  }
  return motion;
}

/** The transition at the cut into the second play, as generatedTransitionEffects and the flash fades draw it. */
function transitionMotion({ transition, t, cut }: { transition: EditConfig['transition']; t: number; cut: number }): Motion {
  const motion: Motion = { zoom: 1, shakeX: 0, shakeY: 0, glitch: 0, grade: false, flash: null };
  const into = t - cut;
  if (transition === 'flash') {
    if (within(into, [-FLASH_HALF_SECONDS, 0])) motion.flash = { color: 'white', opacity: (into + FLASH_HALF_SECONDS) / FLASH_HALF_SECONDS };
    if (within(into, [0, FLASH_HALF_SECONDS])) motion.flash = { color: 'white', opacity: 1 - into / FLASH_HALF_SECONDS };
  }
  if (transition === 'whip') {
    motion.zoom = 1 + 0.12 * envelope(into, { start: 0, peak: 0.08, end: 0.22 });
    if (within(into, [0, 0.12])) motion.flash = { color: 'white', opacity: 0.12 };
  }
  if (transition === 'zoom-whip') {
    motion.zoom = 1 + 0.55 * envelope(into, { start: 0, peak: 0.02, end: 0.28 });
    if (within(into, [0, 0.1])) motion.flash = { color: 'white', opacity: 0.14 };
  }
  if (transition === 'dip') {
    if (within(into, [0, 0.18])) motion.flash = { color: 'black', opacity: 0.32 };
    if (within(into, [-0.22, 0])) motion.flash = { color: 'black', opacity: 0.34 };
  }
  if (transition === 'glitch') {
    if (within(into, [0, 0.16]) || within(into, [-0.14, 0])) motion.glitch = 10;
    if (within(into, [0, 0.1])) motion.flash = { color: '#00ffff', opacity: 0.16 };
    if (within(into, [-0.12, 0])) motion.flash = { color: '#00ffff', opacity: 0.14 };
  }
  return motion;
}

function milestone(kills: number): string {
  return ['', '', '2K', '3K', '4K', 'ACE'][kills] ?? '';
}

function counterAt(timeline: PreviewTimeline, t: number): PreviewFrame['counter'] {
  const { edit, kills } = timeline;
  if (!edit.killCounter) return null;
  for (const [i, k] of kills.entries()) {
    const next = kills[i + 1];
    const end = next !== undefined ? Math.min(k + 0.95, next) : k + 0.95;
    const opacity = fade(t, { start: k + 0.05, end, fadeOut: 0.25 });
    if (opacity === 0) continue;
    const last = i === kills.length - 1 && !edit.playBadges ? milestone(kills.length) : '';
    return { value: last || String(i + 1), size: last ? 72 : 60, opacity };
  }
  return null;
}

function badgeAt(timeline: PreviewTimeline, t: number): PreviewFrame['badge'] {
  if (!timeline.edit.playBadges) return null;
  const start = (timeline.kills[1] ?? 0) + 0.05;
  const end = Math.min(start + PLAY_BADGE_SECONDS, timeline.cut);
  const opacity = fade(t, { start, end, fadeIn: 0.08, fadeOut: 0.25 });
  return opacity > 0 ? { label: '2K', opacity } : null;
}

function phaseAt(timeline: PreviewTimeline, t: number): string {
  const { edit, statsStart, cut, kills } = timeline;
  const clip = clipAt(timeline, t);
  if (statsStart !== null && t >= statsStart) return 'Tarjeta final con las estadísticas';
  if (edit.outro && t >= timeline.duration - OUTRO_SECONDS) return 'Cierre';
  if (edit.transition !== 'cut' && within(t, [cut - 0.3, cut + 0.35])) return `Transición: ${TRANSITION_NAME[edit.transition]}`;
  if (clip === 'replay') return `Repetición desde la víctima a ${String(edit.victimReplaySpeed ?? 0.5).replace('.', ',')}x`;
  if (kills.some((k) => within(t, [k - 0.25, k + 0.5]))) return `Efecto de kill: ${EFFECT_NAME[edit.killEffect]}`;
  if (edit.intro && t < INTRO_SECONDS) return 'Apertura';
  if (edit.hookText && t < HOOK_SECONDS) return 'Título automático';
  return clip === 'a' ? 'Jugada 1' : 'Jugada 2';
}

function clipAt({ cut, replay }: PreviewTimeline, t: number): PreviewClip {
  if (t >= cut) return 'b';
  return replay && t >= replay.start ? 'replay' : 'a';
}

/** When the opening card or the title leaves the screen. */
function titleEnd(edit: EditConfig): number {
  if (edit.intro) return INTRO_SECONDS;
  return edit.hookText ? HOOK_SECONDS : 0;
}

/** The preview frame at second t of the example Short. */
export function previewFrame(timeline: PreviewTimeline, t: number): PreviewFrame {
  const { edit, cut, kills, duration, statsStart } = timeline;
  const clip = clipAt(timeline, t);
  const effectKills = clip === 'replay' ? [] : kills;
  const motions = [...effectKills.map((k) => killMotion({ effect: edit.killEffect, t, kill: k })), transitionMotion({ transition: edit.transition, t, cut })];
  const flash = motions.reduce<PreviewFlash | null>((current, motion) => motion.flash ?? current, null);
  const hookEnd = titleEnd(edit);
  const stats = statsStart === null ? 0 : fade(t, { start: statsStart, end: duration, fadeIn: 0.2 });
  const winProbability = edit.winProbability ? fade(t, { start: Math.max(cut + 0.2, hookEnd), end: statsStart ?? duration, fadeIn: 0.15 }) : 0;
  return {
    clip,
    zoom: Math.max(...motions.map((motion) => motion.zoom)),
    shakeX: motions.reduce((sum, motion) => sum + motion.shakeX, 0),
    shakeY: motions.reduce((sum, motion) => sum + motion.shakeY, 0),
    glitch: Math.max(...motions.map((motion) => motion.glitch)),
    grade: motions.some((motion) => motion.grade),
    flash,
    counter: stats > 0 ? null : counterAt(timeline, t),
    badge: stats > 0 ? null : badgeAt(timeline, t),
    notices: clip === 'replay' ? 0 : kills.filter((k) => within(t, [k, k + NOTICE_SECONDS]) && (k < cut) === (t < cut)).length,
    hook: edit.hookText && !edit.intro ? fade(t, { start: 0, end: HOOK_SECONDS, fadeIn: 0.12, fadeOut: 0.35 }) : 0,
    intro: edit.intro ? fade(t, { start: 0, end: INTRO_SECONDS, fadeIn: 0.1, fadeOut: 0.25 }) : 0,
    outro: edit.outro ? fade(t, { start: duration - OUTRO_SECONDS, end: duration, fadeIn: 0.25 }) : 0,
    stats,
    winProbability,
    verticalHud: t >= hookEnd && clip !== 'replay' && stats === 0,
    phase: phaseAt(timeline, t),
  };
}
