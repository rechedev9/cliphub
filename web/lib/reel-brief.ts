import { affiliateFamilyLabel, affiliateStyleLabel, type EditConfig, type Preset } from './api/types.ts';
import { NATIVE_HUD_LABEL } from './preset-copy.ts';
import { resolvedReelFormat } from './reel-format.ts';

export type CreativeBriefItem = {
  label: string;
  value: string;
};

export type MusicBrief =
  | { status: 'pending' }
  | { status: 'none' }
  | { status: 'track'; title: string; volumePercent: number; gameVolumePercent: number };

export function canForgeReel({
  creating,
  hasPreset,
  selectionCount,
  musicDecided,
}: {
  creating: boolean;
  hasPreset: boolean;
  selectionCount: number;
  musicDecided: boolean;
}): boolean {
  return !creating && hasPreset && selectionCount > 0 && musicDecided;
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
  return text?.trim() ? `Sí · “${text.trim()}”` : `Sí · ${generatedFallback}`;
}

export function musicBriefValue(music: MusicBrief): string {
  if (music.status === 'pending') return 'Pendiente de decisión';
  if (music.status === 'none') return 'Sin música';
  return `${music.title} · música ${music.volumePercent}% · juego ${music.gameVolumePercent}%`;
}

export function isLandscapeRecap(edit: Pick<EditConfig, 'format' | 'matchRecap'>): boolean {
  return edit.format === 'landscape-16x9' && edit.matchRecap;
}

export function constrainEditConfig(edit: EditConfig): EditConfig {
  if (edit.format !== 'short-9x16') return edit;
  if (!edit.matchRecap && !edit.voiceComms && !edit.nativeHud && !edit.demoSource && !edit.overlayTheme) return edit;
  const next: EditConfig = { ...edit, matchRecap: false, voiceComms: false, nativeHud: false };
  delete next.demoSource;
  delete next.overlayTheme;
  return next;
}

/** The exact values used for capture and rendering. */
export function reelCreativeBrief(
  raw: EditConfig,
  preset: Preset | null,
  music: MusicBrief,
): CreativeBriefItem[] {
  const edit = constrainEditConfig(raw);
  let hud = 'Pendiente de preset';
  if (edit.nativeHud) {
    hud = NATIVE_HUD_LABEL;
  } else if (preset?.hudMode) {
    hud = HUD_LABEL[preset.hudMode] ?? `Modo ${preset.hudMode}`;
  }
  return [
    { label: 'Formato', value: FORMAT_LABEL[resolvedReelFormat(edit.format, preset)] },
    { label: 'Entrega', value: isLandscapeRecap(edit) ? 'POV landscape · rondas en vivo (sin freeze)' : 'Compilado de jugadas' },
    {
      label: 'Comms',
      value: edit.voiceComms
        ? `Mezclar comms del equipo · ${Math.round((edit.voiceVolume ?? 0.85) * 100)}%`
        : 'Sin comms',
    },
    { label: 'HUD / killfeed', value: hud },
    { label: 'Efecto de kill', value: EFFECT_LABEL[edit.killEffect] },
    { label: 'Transición', value: TRANSITION_LABEL[edit.transition] },
    { label: 'Título / contador', value: `${edit.hookText ? 'Título automático' : 'Sin título automático'} · ${edit.killCounter ? 'Contador activado' : 'Sin contador'}` },
    { label: 'Intro', value: bookendLabel(edit.intro, edit.introText, 'titular generado') },
    { label: 'Outro', value: bookendLabel(edit.outro, edit.outroText, 'firma ClipHub') },
    {
      label: 'Afiliado',
      value: edit.keyDropStyle
        ? `${affiliateFamilyLabel(edit.keyDropFamily ?? '', edit.keyDropStyle)} · ${affiliateStyleLabel(edit.keyDropFamily ?? '', edit.keyDropStyle)} · ${(edit.keyDropCode?.trim() || 'ZACKCSGO').toUpperCase()} · ${(edit.keyDropStartSeconds ?? 0).toFixed(1)}s–${edit.keyDropEndSeconds != null ? edit.keyDropEndSeconds.toFixed(1) : 'fin'}s`
        : 'No',
    },
    { label: 'Música', value: musicBriefValue(music) },
    {
      label: 'Portada',
      value: edit.coverStrategy === 'generated-gameplay'
        ? 'Generar candidatos de gameplay para revisión'
        : 'No generar portada',
    },
  ];
}
