import { customHudLabel } from '../custom-hud.ts';
import { bumperSummary, type FullDemoOptions } from '../full-demo-plan.ts';
import { INTRO_DESIGN_LABEL, INTRO_DESIGNS } from '../overlay-design-gallery.ts';
import type { CreativeBriefItem } from '../reel-brief.ts';
import { FULL_DEMO_PRO_SETTINGS_LABEL, proSettingsSummary } from './full-demo-pro-settings.ts';

type SourceKind = FullDemoOptions['source_kind'];

/** The form controls and the summary share these words, so the recap reads like the settings. */
export const FULL_DEMO_SOURCE_LABEL = 'Origen de la demo';
export const FULL_DEMO_SOURCES: readonly { value: SourceKind; label: string }[] = [
  { value: 'faceit', label: 'FACEIT' }, { value: 'premier', label: 'Premier' }, { value: 'professional', label: 'Profesional' }, { value: 'demo', label: 'Demo local' },
];
export const FULL_DEMO_BUMPERS_TITLE = 'Intro, sponsor y outro';
export const FULL_DEMO_TRUEVIEW_LABEL = 'POV original 1:1';
export const FULL_DEMO_RADAR_LABEL = 'Radar de ClipHub';
export const FULL_DEMO_VOICE_LABEL = 'Incluir voces del equipo';
export const FULL_DEMO_TRANSITIONS_LABEL = 'Activar efectos entre rondas';
export const FULL_DEMO_HOOK_LABEL = 'Gancho inicial';
export const FULL_DEMO_HOOK_NOTE = 'El vídeo abre con unos segundos de la mejor jugada de la partida, antes de la primera ronda.';
export const FULL_DEMO_CAMERA_LABEL = 'Segunda cámara';
export const FULL_DEMO_CAMERA_NOTE = 'Una ventana pequeña sobre la partida con lo que veía un rival que aguantaba un ángulo, en los 5 segundos antes de que el jugador lo eliminara.';
export const FULL_DEMO_HALFTIME_LABEL = 'Marcador del descanso';
export const FULL_DEMO_HALFTIME_NOTE = 'Al empezar la segunda parte, una tarjeta con los dos equipos y el resultado del descanso durante unos segundos.';
export const FULL_DEMO_SUBSCRIBE_LABEL = 'Botón de suscripción';
export const FULL_DEMO_SUBSCRIBE_NOTE = 'Un botón «Suscríbete» con campana en la esquina inferior derecha, dos veces durante las rondas.';

export type VoiceStatus = { text: string; problem: boolean };

const VOICE_PENDING = 'Las voces del equipo se comprueban al preparar el vídeo.';
const VOICE_STATUS: Record<string, VoiceStatus> = {
  available: { text: 'Voces del equipo disponibles en esta demo.', problem: false },
  // Planned while voices were off: the next preparation checks them.
  not_requested: { text: VOICE_PENDING, problem: false },
  no_packets: { text: 'Esta demo no contiene voces.', problem: true },
  no_team_voice: { text: 'Esta demo no tiene voces del equipo del jugador.', problem: true },
  silent: { text: 'Las voces del equipo están en silencio en esta demo.', problem: true },
  unsupported: { text: 'El formato de voz de esta demo no es compatible.', problem: true },
  unsupported_codec: { text: 'El códec de voz de esta demo no es compatible.', problem: true },
  decode_failed: { text: 'No se pudieron leer las voces de esta demo.', problem: true },
  // Read from the library demo by the Subtítulos panel, before any plan.
  extracting: { text: 'Leyendo las voces de la demo…', problem: false },
  demo_missing: { text: 'El archivo de esta demo ya no está en este PC.', problem: true },
};

/** What the last plan found about the demo's voices; `null` means no plan has analysed it yet. */
export function fullDemoVoiceStatus(availability: string | null): VoiceStatus {
  if (availability === null) return { text: VOICE_PENDING, problem: false };
  return VOICE_STATUS[availability] ?? { text: `Estado de las voces: ${availability}.`, problem: true };
}

export function demoHasNoVoices(availability: string | null): boolean {
  return availability === 'no_packets';
}

function voiceValue(enabled: boolean, availability: string | null): string {
  if (demoHasNoVoices(availability)) return 'No hay voces en esta demo';
  if (!enabled) return 'Sin voces';
  if (availability === 'available') return 'Incluidas';
  return fullDemoVoiceStatus(availability).problem ? 'No se pueden usar en esta demo' : 'Si la demo las tiene';
}

function subtitlesValue(options: FullDemoOptions, voiceAvailability: string | null): string {
  const subtitles = options.subtitles;
  if (!subtitles?.enabled || !options.audio.voice.enabled || demoHasNoVoices(voiceAvailability)) return 'No';
  const hidden = subtitles.hidden?.length ?? 0;
  return `Sí${hidden > 0 ? `, ${hidden} ${hidden === 1 ? 'frase oculta' : 'frases ocultas'}` : ''}`;
}

function sourceValue(options: FullDemoOptions): string {
  const label = FULL_DEMO_SOURCES.find((entry) => entry.value === options.source_kind)?.label ?? options.source_kind;
  return options.source_kind === 'demo' && options.overlays.source === 'faceit' ? `${label}, con datos de FACEIT` : label;
}

function hasBumpers(options: FullDemoOptions): boolean {
  const bumpers = options.bumpers;
  return Boolean(bumpers?.intro.enabled || bumpers?.sponsor?.enabled || bumpers?.outro.enabled);
}

function hookValue(enabled: boolean, skipped: boolean): string {
  if (!enabled) return 'No';
  return skipped ? 'Omitido' : 'Sí';
}

/** The recap rows after player and rounds, labelled as the controls that set them. */
export function fullDemoBriefItems(options: FullDemoOptions, voiceAvailability: string | null, hookSkipped = false, cameraSkipped = false): CreativeBriefItem[] {
  const hudName = options.overlays.hud_custom?.name ?? customHudLabel(options.overlays.hud_theme);
  return [
    { label: 'HUD', value: hudName },
    { label: FULL_DEMO_TRUEVIEW_LABEL, value: options.capture.trueview ? 'Sí' : 'No' },
    { label: FULL_DEMO_RADAR_LABEL, value: options.capture.cliphub_radar ? 'Sí' : 'No' },
    { label: FULL_DEMO_PRO_SETTINGS_LABEL, value: proSettingsSummary(options.capture.pro_settings) },
    { label: 'Voces del equipo', value: voiceValue(options.audio.voice.enabled, voiceAvailability) },
    { label: 'Subtítulos', value: subtitlesValue(options, voiceAvailability) },
    { label: 'Efectos entre rondas', value: (options.transitions?.enabled ?? true) ? 'Activados' : 'Desactivados' },
    { label: FULL_DEMO_HOOK_LABEL, value: hookValue(options.hook?.enabled === true, hookSkipped) },
    { label: FULL_DEMO_CAMERA_LABEL, value: hookValue(options.second_camera?.enabled === true, cameraSkipped) },
    { label: FULL_DEMO_BUMPERS_TITLE, value: hasBumpers(options) ? bumperSummary(options) : 'Ninguno' },
    { label: INTRO_DESIGN_LABEL, value: options.overlays.theme === 'hud' ? INTRO_DESIGNS.hud : INTRO_DESIGNS.neon },
    { label: FULL_DEMO_SOURCE_LABEL, value: sourceValue(options) },
    { label: FULL_DEMO_HALFTIME_LABEL, value: options.overlays.halftime ? 'Sí' : 'No' },
    { label: FULL_DEMO_SUBSCRIBE_LABEL, value: options.overlays.subscribe ? 'Sí' : 'No' },
  ];
}
