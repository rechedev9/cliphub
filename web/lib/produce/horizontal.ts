import type { EditConfig } from '../api/types.ts';
import { PRODUCE_SUMMARY_CTA } from './copy.ts';
import { shortEditNeedsCapture, shortSettingsFromVersion, versionDateLabel, type ShortCaptureChoice } from './edit-capture.ts';
import type { ShortSettings } from './short-draft.ts';

/** Crear horizontal: the 16:9 summary of a Short's plays, as `GET .../renders/{variant}/horizontal` builds it. */
export type HorizontalDraft = {
  settings: ShortSettings;
  /** The Short's plays whose recordings are still on disk. */
  recordedIds: string[];
  missingIds: string[];
  /** The 9:16 options the summary leaves out, as the user reads them. */
  dropped: string[];
  date: string;
};

const NAME = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
const VARIANT = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;

const DROPPED_LABEL: Readonly<Record<string, string>> = {
  vertical_hud: 'HUD vertical',
  centered_killfeed: 'kill feed centrada',
  stats_card: 'tarjeta de estadísticas',
  radar: 'radar de ClipHub',
  kill_counter: 'contador de kills',
  player_cards: 'cartas de jugador',
};

function field(body: unknown, key: string): unknown {
  return typeof body === 'object' && body !== null ? Reflect.get(body, key) : undefined;
}

function strings(value: unknown, pattern: RegExp): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && pattern.test(item)) : [];
}

function listLabel(items: readonly string[]): string {
  return items.length <= 1 ? items.join('') : `${items.slice(0, -1).join(', ')} y ${items[items.length - 1]}`;
}

/** The server draft as producer settings; null when it has no plays or is not a 16:9 summary. */
export function projectHorizontalDraft(body: unknown): HorizontalDraft | null {
  const preset = field(body, 'preset');
  const segmentIds = strings(field(body, 'segment_ids'), NAME);
  if (typeof preset !== 'string' || !VARIANT.test(preset) || segmentIds.length === 0) return null;
  const settings = shortSettingsFromVersion({ variant: preset, segmentIds, edit: field(body, 'edit'), music: field(body, 'music') });
  if (settings === null || settings.editConfig.format !== 'landscape-16x9') return null;
  const capture = field(body, 'capture');
  const missing = new Set(field(capture, 'needed') === true ? strings(field(capture, 'missing_segment_ids'), NAME) : []);
  const createdAt = field(body, 'created_at');
  return {
    settings,
    recordedIds: segmentIds.filter((id) => !missing.has(id)),
    missingIds: segmentIds.filter((id) => missing.has(id)),
    dropped: strings(field(body, 'dropped'), NAME).flatMap((id) => DROPPED_LABEL[id] ?? []),
    date: typeof createdAt === 'string' ? versionDateLabel(createdAt) : '-',
  };
}

function captureStyle(edit: EditConfig): string {
  return edit.nativeHud ? 'gameplay' : edit.captureHud ?? 'gameplay';
}

/** The Short's kill feed framing holds only while the video keeps the Short's capture style. */
export function withShortFraming(edit: EditConfig, from: { settings: Pick<ShortSettings, 'editConfig'> } | null): EditConfig {
  const source = from?.settings.editConfig;
  const keep = source?.portraitSafeKillfeed === true && edit.format === 'landscape-16x9' && captureStyle(edit) === captureStyle(source);
  if (keep === (edit.portraitSafeKillfeed === true)) return edit;
  const next: EditConfig = { ...edit };
  if (keep) next.portraitSafeKillfeed = true;
  else delete next.portraitSafeKillfeed;
  return next;
}

/** The draft records nothing; plays whose clips are gone, new plays or another capture do. */
export function horizontalNeedsCapture(draft: HorizontalDraft, next: ShortCaptureChoice): boolean {
  return shortEditNeedsCapture({ playIds: draft.recordedIds, hudMode: next.hudMode, edit: draft.settings.editConfig }, next);
}

export function horizontalCtaLabel(needsCapture: boolean | null): string {
  if (needsCapture === null) return PRODUCE_SUMMARY_CTA;
  return needsCapture ? 'Grabar y montar el horizontal' : 'Montar el horizontal';
}

type NoticeInput = { draft: HorizontalDraft; needsCapture: boolean | null; playIds: readonly string[] };

/** The Short's plays chosen whose recordings are gone. */
export function horizontalGonePlays({ draft, playIds }: Pick<NoticeInput, 'draft' | 'playIds'>): number {
  return playIds.filter((id) => draft.missingIds.includes(id)).length;
}

function captureSentence({ draft, needsCapture, playIds }: NoticeInput): string | null {
  if (needsCapture === null) return null;
  if (!needsCapture) return 'Se monta con lo ya grabado, sin abrir CS2.';
  const gone = horizontalGonePlays({ draft, playIds });
  if (gone === 0) return 'Estos cambios necesitan grabar con CS2.';
  if (draft.recordedIds.length === 0) return 'Las grabaciones del Short ya no están en este PC: se graban otra vez con CS2.';
  return gone === 1
    ? 'La grabación de 1 jugada del Short ya no está en este PC: se graba otra vez con CS2.'
    : `Las grabaciones de ${gone} jugadas del Short ya no están en este PC: se graban otra vez con CS2.`;
}

/** What the producer says over a horizontal: where it comes from, what it drops and whether CS2 opens. */
export function horizontalNotice(input: NoticeInput): string {
  const parts = [`Creas el horizontal del Short del ${input.draft.date} con sus jugadas en el mismo orden, su captura, su música y sus efectos.`];
  if (input.draft.dropped.length > 0) parts.push(`El horizontal no lleva estas opciones del vertical: ${listLabel(input.draft.dropped)}.`);
  parts.push('El Short no cambia.');
  const capture = captureSentence(input);
  if (capture !== null) parts.push(capture);
  return parts.join(' ');
}
