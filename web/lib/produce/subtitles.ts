import type { EditConfig, Play } from '../api/types.ts';
import type { FullDemoOptions } from '../full-demo-plan.ts';
import type { TranscriberState, VoiceTranscriber, VoiceTranscriptState } from '../api/voice.ts';
import { fullDemoVoiceStatus } from './full-demo-brief.ts';

/** One phrase of the voices a demo recorded (`library.DemoVoiceLine` in Go); `start_ms` is demo time. */
export type DemoVoiceLine = {
  id: string;
  match_id: string;
  steamid64: string;
  name: string;
  team_key: string;
  start_ms: number;
  end_ms: number;
  tick: number;
  round: number;
  text: string;
  corrected: boolean;
  english?: string;
};

/** GET /api/demos/{jobId}/subtitles: the demo transcript with the phrases of the player's team only. */
export type JobSubtitles = {
  match_id: string;
  state: VoiceTranscriptState;
  english: VoiceTranscriptState;
  error?: string;
  english_error?: string;
  progress?: number;
  /** The Full Demo voice check's words, plus extracting and demo_missing. */
  availability: string;
  lines: DemoVoiceLine[];
  transcriber: VoiceTranscriber;
  translator: VoiceTranscriber;
};

/** The subtitles of one video; hidden phrases are left out of this video only. */
export type SubtitleChoice = { enabled: boolean; english: boolean; hidden: readonly string[] };

/** Where the video plays on the demo: the chosen plays in demo seconds, or rounds (null is every round). */
export type PhraseScope =
  | { kind: 'seconds'; windows: readonly { start: number; end: number }[] }
  | { kind: 'rounds'; rounds: readonly number[] | null };

export type SubtitleRow = { id: string; speaker: string; round: number; text: string; corrected: boolean; hidden: boolean };

export type SubtitleAvailability = { enabled: boolean; reason: string | null };

export const SUBTITLES_LABEL = 'Subtítulos';
export const SUBTITLES_ENGLISH_LABEL = 'Traducir al inglés';

export function subtitleAvailability(input: { voices: boolean; voicesLabel: string; availability: string | null }): SubtitleAvailability {
  if (!input.voices) return { enabled: false, reason: `Activa «${input.voicesLabel}»: los subtítulos escriben lo que dicen las voces.` };
  const status = fullDemoVoiceStatus(input.availability);
  return status.problem ? { enabled: false, reason: status.text } : { enabled: true, reason: null };
}

function inScope(line: DemoVoiceLine, scope: PhraseScope): boolean {
  if (scope.kind === 'rounds') return scope.rounds === null || scope.rounds.includes(line.round);
  const second = line.start_ms / 1000;
  return scope.windows.some((window) => second >= window.start && second < window.end);
}

/** The phrases the video can show, in the order they were said; with English, the text is the translation. */
export function subtitleRows(lines: readonly DemoVoiceLine[], scope: PhraseScope, choice: SubtitleChoice): SubtitleRow[] {
  return lines
    .filter((line) => inScope(line, scope))
    .toSorted((a, b) => a.start_ms - b.start_ms)
    .map((line) => ({
      id: line.id,
      speaker: line.name,
      round: line.round,
      text: choice.english ? line.english ?? '' : line.text,
      corrected: line.corrected,
      hidden: choice.hidden.includes(line.id),
    }));
}

/** The chosen plays on the demo clock; plays of an old analysis without it fall back to their rounds. */
export function playScope(plays: readonly Pick<Play, 'round' | 'startSeconds' | 'endSeconds'>[]): PhraseScope {
  const windows = plays.flatMap((play) => play.startSeconds === undefined || play.endSeconds === undefined ? [] : [{ start: play.startSeconds, end: play.endSeconds }]);
  if (windows.length === plays.length) return { kind: 'seconds', windows };
  return { kind: 'rounds', rounds: [...new Set(plays.map((play) => play.round))] };
}

export function editSubtitles(edit: EditConfig): SubtitleChoice {
  return { enabled: edit.subtitles === true, english: edit.subtitlesEnglish === true, hidden: edit.subtitlesHidden ?? [] };
}

/** The edit with this choice; off leaves no subtitle field behind. */
export function withSubtitles(edit: EditConfig, choice: SubtitleChoice): EditConfig {
  const { subtitles: _on, subtitlesEnglish: _english, subtitlesHidden: _hidden, ...rest } = edit;
  if (!choice.enabled) return rest;
  return { ...rest, subtitles: true, ...(choice.english ? { subtitlesEnglish: true } : {}), ...(choice.hidden.length > 0 ? { subtitlesHidden: [...choice.hidden] } : {}) };
}

/** The panel header of a Short: off, on, or in English. */
export function subtitlesSummary(choice: SubtitleChoice, voices: boolean): string {
  if (!choice.enabled || !voices) return 'No';
  return choice.english ? 'En inglés' : 'Activados';
}

export function fullDemoSubtitles(options: FullDemoOptions): SubtitleChoice {
  const subtitles = options.subtitles;
  return { enabled: subtitles?.enabled === true, english: subtitles?.english === true, hidden: subtitles?.hidden ?? [] };
}

export function withFullDemoSubtitles(options: FullDemoOptions, choice: SubtitleChoice): FullDemoOptions {
  const { subtitles: _old, ...rest } = options;
  return choice.enabled ? { ...rest, subtitles: { enabled: true, english: choice.english, hidden: [...choice.hidden] } } : rest;
}

export function toggleHiddenPhrase(choice: SubtitleChoice, id: string): SubtitleChoice {
  const hidden = choice.hidden.includes(id) ? choice.hidden.filter((other) => other !== id) : [...choice.hidden, id].toSorted();
  return { ...choice, hidden };
}

function percent(progress: number | undefined): string {
  return `${Math.round((progress ?? 0) * 100)} %`;
}

/** What the render will do with the transcript as it stands; null when the subtitles are ready. */
export function subtitleNote(subs: JobSubtitles, english: boolean): string | null {
  if (subs.transcriber.state !== 'installed') return 'Instala la transcripción para crear los subtítulos. Sin ella, el vídeo sale sin subtítulos.';
  if (english && subs.translator.state !== 'installed') return 'Instala la traducción para los subtítulos en inglés. Sin ella, el vídeo sale sin subtítulos.';
  switch (subs.state) {
    case 'none':
      return 'Las voces aún no están transcritas: se transcriben al crear el vídeo. Si no terminan antes del montaje, el vídeo sale sin subtítulos.';
    case 'queued':
    case 'running':
      return `Transcribiendo las voces… ${percent(subs.progress)}. Si no termina antes del montaje, el vídeo sale sin subtítulos.`;
    case 'failed':
      return 'La transcripción falló, así que el vídeo saldrá sin subtítulos. Prueba a transcribir de nuevo.';
    case 'ready':
      break;
  }
  if (!english || subs.english === 'ready') return null;
  if (subs.english === 'queued' || subs.english === 'running') return `Traduciendo al inglés… ${percent(subs.progress)}. Si no termina antes del montaje, el vídeo sale sin subtítulos.`;
  if (subs.english === 'failed') return 'La traducción falló, así que el vídeo saldrá sin subtítulos. Prueba a traducir de nuevo.';
  return 'Las frases aún no están en inglés: se traducen al crear el vídeo. Si no terminan antes del montaje, el vídeo sale sin subtítulos.';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

const TRANSCRIPT_STATES: readonly string[] = ['none', 'queued', 'running', 'ready', 'failed'];
const INSTALL_STATES: readonly string[] = ['not_installed', 'installing', 'installed', 'failed', 'unsupported'];

function isTranscriptState(value: unknown): value is VoiceTranscriptState {
  return typeof value === 'string' && TRANSCRIPT_STATES.includes(value);
}

function isInstallState(value: unknown): value is TranscriberState {
  return typeof value === 'string' && INSTALL_STATES.includes(value);
}

function optionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === 'string';
}

function isInstall(value: unknown): value is VoiceTranscriber {
  return isRecord(value) && isInstallState(value.state) && typeof value.version === 'string' && typeof value.model === 'string'
    && typeof value.download_bytes === 'number' && (value.done_bytes === undefined || typeof value.done_bytes === 'number')
    && optionalString(value.error_code) && optionalString(value.error);
}

function isLine(value: unknown): value is DemoVoiceLine {
  if (!isRecord(value)) return false;
  const strings = [value.id, value.match_id, value.steamid64, value.name, value.team_key, value.text];
  const numbers = [value.start_ms, value.end_ms, value.tick, value.round];
  return strings.every((field) => typeof field === 'string') && numbers.every((field) => typeof field === 'number')
    && typeof value.corrected === 'boolean' && optionalString(value.english);
}

function isJobSubtitles(value: unknown): value is JobSubtitles {
  return isRecord(value) && typeof value.match_id === 'string' && isTranscriptState(value.state) && isTranscriptState(value.english)
    && optionalString(value.error) && optionalString(value.english_error) && (value.progress === undefined || typeof value.progress === 'number')
    && typeof value.availability === 'string' && Array.isArray(value.lines) && value.lines.every(isLine)
    && isInstall(value.transcriber) && isInstall(value.translator);
}

/** The fields the panel reads; null for an answer this build does not understand. */
export function parseJobSubtitles(value: unknown): JobSubtitles | null {
  if (!isJobSubtitles(value)) return null;
  const { match_id, state, english, error, english_error, progress, availability, lines, transcriber, translator } = value;
  return {
    match_id, state, english, availability, lines, transcriber, translator,
    ...(error === undefined ? {} : { error }),
    ...(english_error === undefined ? {} : { english_error }),
    ...(progress === undefined ? {} : { progress }),
  };
}

export async function fetchJobSubtitles(jobId: string, signal?: AbortSignal): Promise<JobSubtitles> {
  const response = await fetch(`/api/demos/${encodeURIComponent(jobId)}/subtitles`, { cache: 'no-store', signal });
  if (!response.ok) throw new Error(`subtitles ${response.status}`);
  const subs = parseJobSubtitles(await response.json());
  if (subs === null) throw new Error('subtitles: unexpected answer');
  return subs;
}
