import { affiliateFamilyLabel, affiliateStyleLabel, type EditConfig, type Preset } from './api/types.ts';
import { verticalHudTheme } from './custom-hud.ts';
import { NATIVE_HUD_LABEL } from './preset-copy.ts';
import { formatDecimal } from './format.ts';
import { resolvedReelFormat } from './reel-format.ts';

export type CreativeBriefItem = {
  label: string;
  value: string;
};

export type MusicBrief =
  | { status: 'none' }
  | { status: 'track'; title: string; volumePercent: number; gameVolumePercent: number };

/** Music is optional: a Short without a track goes out "Sin música". */
export function canForgeReel({
  creating,
  hasPreset,
  selectionCount,
}: {
  creating: boolean;
  hasPreset: boolean;
  selectionCount: number;
}): boolean {
  return !creating && hasPreset && selectionCount > 0;
}

/** Library music rerender: the capture already exists; only a changed mix may proceed. */
export function canRerenderWithMusic({
  busy,
  musicChanged,
}: {
  busy: boolean;
  musicChanged: boolean;
}): boolean {
  return !busy && musicChanged;
}

const FORMAT_LABEL: Record<EditConfig['format'], string> = {
  'short-9x16': 'Vertical 9:16 · 1080×1920',
  'landscape-16x9': 'Horizontal 16:9 · 1920×1080',
};

const EFFECT_LABEL: Record<EditConfig['killEffect'], string> = {
  clean: 'Limpio',
  'punch-in': 'Impacto / punch-in',
  velocity: 'Velocidad',
  'freeze-flash': 'Congelado con flash',
  shake: 'Terremoto',
  glitch: 'Glitch',
};

const TRANSITION_LABEL: Record<EditConfig['transition'], string> = {
  cut: 'Corte',
  flash: 'Destello',
  whip: 'Barrido',
  dip: 'Fundido',
  glitch: 'Glitch',
  'zoom-whip': 'Zoom-whip',
};

const HUD_LABEL: Record<string, string> = {
  deathnotices: 'Sin HUD, conserva killfeed',
  clean: 'Sin HUD ni killfeed',
  gameplay: 'HUD completo con killfeed',
};

function bookendLabel(enabled: boolean, text: string | undefined, generatedFallback: string): string {
  if (!enabled) return 'No';
  return text?.trim() ? `Sí · «${text.trim()}»` : `Sí · ${generatedFallback}`;
}

function musicBriefValue(music: MusicBrief): string {
  if (music.status === 'none') return 'Sin música';
  return `${music.title} · música ${music.volumePercent} % · juego ${music.gameVolumePercent} %`;
}

export function isLandscapeRecap(edit: Pick<EditConfig, 'format' | 'matchRecap'>): boolean {
  return edit.format === 'landscape-16x9' && edit.matchRecap;
}

/** A 16:9 video of chosen plays, such as the best moments of several teammates, not a whole match. */
export function isLandscapeHighlights(edit: Pick<EditConfig, 'format' | 'matchRecap'>): boolean {
  return edit.format === 'landscape-16x9' && !edit.matchRecap;
}

export function constrainEditConfig(edit: EditConfig): EditConfig {
  // The hook, the counter and the centered kill feed tell kills; Studio drops them from a death reel too.
  if (edit.deathReel && (edit.hookText || edit.killCounter || edit.centeredKillfeed || edit.verticalHud || edit.playBadges || edit.winProbability)) {
    const next: EditConfig = { ...edit, hookText: false, killCounter: false };
    delete next.centeredKillfeed;
    delete next.verticalHud;
    delete next.verticalHudTheme;
    delete next.playBadges;
    delete next.winProbability;
    return constrainEditConfig(next);
  }
  if (edit.format !== 'short-9x16') {
    if (!edit.statsCard && !edit.verticalHud) return edit;
    // The stats card and the vertical HUD are drawn for 9:16 only.
    const next: EditConfig = { ...edit };
    delete next.statsCard;
    delete next.verticalHud;
    delete next.verticalHudTheme;
    return next;
  }
  if (!edit.matchRecap && !edit.voiceComms && !edit.nativeHud && !edit.demoSource && !edit.overlayTheme) return edit;
  const next: EditConfig = { ...edit, matchRecap: false, voiceComms: false, nativeHud: false };
  delete next.demoSource;
  delete next.overlayTheme;
  return next;
}

/** Full HUD records CS2's own HUD, so Studio refuses the vertical HUD over it. */
export function verticalHudAllowed(preset: Pick<Preset, 'hudMode'> | null): boolean {
  return preset?.hudMode !== 'gameplay';
}

/** The edit that is sent: a vertical HUD the chosen style cannot carry stays in the draft only. */
export function withPresetLimits(edit: EditConfig, preset: Pick<Preset, 'hudMode'> | null): EditConfig {
  if (!edit.verticalHud || verticalHudAllowed(preset)) return edit;
  const next: EditConfig = { ...edit };
  delete next.verticalHud;
  delete next.verticalHudTheme;
  return next;
}

function worldOverlayLabel(edit: EditConfig): string {
  if (edit.matchRecap) return 'No';
  const parts = [edit.playerLabels ? 'Nombres y vida' : '', edit.grenadeTrails ? 'Estelas de granadas' : ''].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : 'No';
}

/** The exact values used for capture and rendering. */
export function reelCreativeBrief(
  raw: EditConfig,
  preset: Preset | null,
  music: MusicBrief,
): CreativeBriefItem[] {
  const edit = withPresetLimits(constrainEditConfig(raw), preset);
  let hud = 'Pendiente de estilo';
  if (edit.nativeHud) {
    hud = NATIVE_HUD_LABEL;
  } else if (preset?.hudMode) {
    hud = HUD_LABEL[preset.hudMode] ?? `Modo ${preset.hudMode}`;
  }
  if (edit.verticalHud && edit.format === 'short-9x16') {
    hud = `${hud} · HUD vertical ${verticalHudTheme(edit.verticalHudTheme).name} con kill feed centrada`;
  } else if (edit.centeredKillfeed && edit.format === 'short-9x16') {
    hud = `${hud} · kill feed centrada arriba`;
  }
  return [
    { label: 'Formato', value: FORMAT_LABEL[resolvedReelFormat(edit.format, preset)] },
    { label: 'Entrega', value: isLandscapeRecap(edit) ? 'POV landscape · rondas en vivo (sin freeze)' : 'Compilado de jugadas' },
    {
      label: 'Comms',
      value: edit.voiceComms
        ? `Mezclar comms del equipo · ${Math.round((edit.voiceVolume ?? 0.85) * 100)} %`
        : 'Sin comms',
    },
    { label: 'HUD / killfeed', value: hud },
    { label: 'Efecto de kill', value: EFFECT_LABEL[edit.killEffect] },
    { label: 'Transición', value: TRANSITION_LABEL[edit.transition] },
    { label: 'Título / contador', value: `${edit.hookText ? 'Título automático' : 'Sin título automático'} · ${edit.killCounter ? 'Contador activado' : 'Sin contador'}` },
    { label: 'Insignias / tarjeta', value: `${edit.playBadges ? 'Insignias de jugada' : 'Sin insignias'} · ${edit.statsCard ? 'Tarjeta de estadísticas' : 'Sin tarjeta'}` },
    { label: 'Prob. de victoria', value: edit.winProbability ? 'Tras el plantado' : 'No' },
    { label: 'Sobre el juego', value: worldOverlayLabel(edit) },
    { label: 'Intro', value: bookendLabel(edit.intro, edit.introText, 'titular generado') },
    { label: 'Outro', value: bookendLabel(edit.outro, edit.outroText, 'firma ClipHub') },
    {
      label: 'Afiliado',
      value: edit.keyDropStyle
        ? `${affiliateFamilyLabel(edit.keyDropFamily ?? '', edit.keyDropStyle)} · ${affiliateStyleLabel(edit.keyDropFamily ?? '', edit.keyDropStyle)} · ${(edit.keyDropCode?.trim() || 'ZACKCSGO').toUpperCase()} · de ${formatDecimal(edit.keyDropStartSeconds ?? 0, 1)} s a ${edit.keyDropEndSeconds != null ? `${formatDecimal(edit.keyDropEndSeconds, 1)} s` : 'el final'}`
        : 'No',
    },
    { label: 'Música', value: musicBriefValue(music) },
    {
      label: 'Portada',
      value: edit.coverStrategy === 'generated-gameplay'
        ? 'Automática (fotograma del vídeo)'
        : 'No generar portada',
    },
  ];
}
