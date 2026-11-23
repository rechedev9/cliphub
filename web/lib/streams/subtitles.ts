import { apiFetch } from '../api/edition-fetch.ts';
import type { VoiceTranscriber, VoiceTranscriptState } from '../api/voice.ts';
import { isInstall, isRecord, isTranscriptState, optionalString } from '../produce/subtitles.ts';
import { isVocabChange, vocabMark, type VocabChange, type VocabMark } from '../produce/vocab-marks.ts';
import { formatStreamClock } from './plan.ts';

export type StreamVoiceLine = {
  id: string;
  start_ms: number;
  end_ms: number;
  text: string;
  corrected: boolean;
  original_text?: string;
  vocab_text?: string;
  vocab_changes?: VocabChange[];
  discarded: boolean;
};

export type StreamVoiceSpan = { start_ms: number; end_ms: number };

export type StreamSubtitles = { shorts: boolean; summary: boolean; hidden: Record<string, string[]> };

export type StreamSubtitlesStale = { shorts: boolean; summary: boolean };

export type StreamTranscript = {
  state: VoiceTranscriptState;
  error?: string;
  progress?: number;
  availability: string;
  spans: StreamVoiceSpan[];
  pending: StreamVoiceSpan[];
  lines: StreamVoiceLine[];
  transcriber: VoiceTranscriber;
  subtitles: StreamSubtitles;
  stale: StreamSubtitlesStale;
};

export type StreamSubtitlesKind = 'shorts' | 'summary';

export type Stretch = { start: number; end: number };

export type StreamSubtitleRow = {
  id: string; clock: string; text: string; corrected: boolean; original: string | null; discarded: boolean; hidden: boolean;
  vocab: VocabMark | null;
};

export const SUMMARY_VIDEO = 'summary';

export function streamSubtitlesStatus(availability: string): { enabled: boolean; note: string | null } {
  switch (availability) {
    case 'available':
      return { enabled: true, note: null };
    case 'no_audio':
      return { enabled: false, note: 'Este vídeo no tiene audio, así que no hay nada que transcribir.' };
    case 'mixed_track':
      return {
        enabled: true,
        note: 'Este stream tiene tu voz y el juego en una sola pista, así que también pueden salir tus compañeros o el anunciador del juego. Descarta las frases que no sean tuyas.',
      };
    default:
      return { enabled: false, note: null };
  }
}

export function streamSubtitleRows(input: { lines: readonly StreamVoiceLine[]; windows: readonly Stretch[]; hidden: readonly string[] }): StreamSubtitleRow[] {
  return input.lines
    .filter((line) => input.windows.some((window) => line.end_ms > window.start * 1000 && line.start_ms < window.end * 1000))
    .toSorted((a, b) => a.start_ms - b.start_ms)
    .map((line) => ({
      id: line.id,
      clock: formatStreamClock(line.start_ms / 1000),
      text: line.text,
      corrected: line.corrected,
      original: line.corrected && line.original_text ? line.original_text : null,
      discarded: line.discarded,
      hidden: input.hidden.includes(line.id),
      vocab: vocabMark({
        text: line.text, heard: line.original_text || line.text, vocabText: line.vocab_text, changes: line.vocab_changes, corrected: line.corrected,
      }),
    }));
}

export function stretchesKey(stretches: readonly Stretch[]): string {
  return stretches.map((stretch) => `${stretch.start.toFixed(3)}-${stretch.end.toFixed(3)}`).toSorted().join('|');
}

export function streamSubtitleNote(transcript: StreamTranscript): string | null {
  switch (transcript.transcriber.state) {
    case 'not_installed':
    case 'installing':
      return 'La transcripción se descarga una sola vez, al usar subtítulos, y después transcribe estos momentos.';
    case 'failed':
    case 'unsupported':
      return 'Sin la transcripción, los vídeos salen sin subtítulos.';
    case 'installed':
      break;
  }
  switch (transcript.state) {
    case 'queued':
    case 'running':
      return `Transcribiendo lo que dices en estos momentos… ${Math.round((transcript.progress ?? 0) * 100)} %. Puedes seguir editando.`;
    case 'failed':
      return 'La transcripción falló en este PC. Prueba a transcribir de nuevo.';
    default:
      return null;
  }
}

function isLine(value: unknown): value is StreamVoiceLine {
  return isRecord(value) && typeof value.id === 'string' && typeof value.start_ms === 'number' && typeof value.end_ms === 'number'
    && typeof value.text === 'string' && typeof value.corrected === 'boolean' && typeof value.discarded === 'boolean' && optionalString(value.original_text)
    && optionalString(value.vocab_text)
    && (value.vocab_changes === undefined || (Array.isArray(value.vocab_changes) && value.vocab_changes.every(isVocabChange)));
}

function isSpan(value: unknown): value is StreamVoiceSpan {
  return isRecord(value) && typeof value.start_ms === 'number' && typeof value.end_ms === 'number';
}

function isIdList(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((id) => typeof id === 'string');
}

function isSubtitles(value: unknown): value is StreamSubtitles {
  return isRecord(value) && typeof value.shorts === 'boolean' && typeof value.summary === 'boolean'
    && isRecord(value.hidden) && Object.values(value.hidden).every(isIdList);
}

export function parseStreamSubtitles(value: unknown): StreamSubtitles | null {
  if (!isSubtitles(value)) return null;
  return { shorts: value.shorts, summary: value.summary, hidden: value.hidden };
}

function parseStale(value: unknown): StreamSubtitlesStale | null {
  if (!isRecord(value) || typeof value.shorts !== 'boolean' || typeof value.summary !== 'boolean') return null;
  return { shorts: value.shorts, summary: value.summary };
}

export function parseStreamTranscript(value: unknown): StreamTranscript | null {
  if (!isRecord(value) || !isTranscriptState(value.state) || typeof value.availability !== 'string' || !optionalString(value.error)) return null;
  const { state, availability, error, progress, spans, pending, lines, transcriber } = value;
  if (progress !== undefined && typeof progress !== 'number') return null;
  if (!Array.isArray(spans) || !spans.every(isSpan) || !Array.isArray(pending) || !pending.every(isSpan)) return null;
  if (!Array.isArray(lines) || !lines.every(isLine) || !isInstall(transcriber)) return null;
  const subtitles = parseStreamSubtitles(value.subtitles);
  const stale = parseStale(value.stale);
  if (subtitles === null || stale === null) return null;
  return {
    state, availability, spans, pending, lines, transcriber, subtitles, stale,
    ...(error === undefined ? {} : { error }),
    ...(progress === undefined ? {} : { progress }),
  };
}

export class StreamSubtitlesError extends Error {
  readonly code: string | undefined;

  constructor(status: number, code: string | undefined) {
    super(`stream subtitles ${status}`);
    this.code = code;
  }
}

async function readAnswer(response: Response): Promise<unknown> {
  const body: unknown = await response.json().catch(() => null);
  if (!response.ok) throw new StreamSubtitlesError(response.status, isRecord(body) && typeof body.code === 'string' ? body.code : undefined);
  return body;
}

function jsonInit(method: string, body: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
}

function transcriptPath(jobId: string): string {
  return `/api/streams/${encodeURIComponent(jobId)}/transcript`;
}

async function readTranscript(response: Response): Promise<StreamTranscript> {
  const transcript = parseStreamTranscript(await readAnswer(response));
  if (transcript === null) throw new Error('stream transcript: unexpected answer');
  return transcript;
}

export async function fetchStreamTranscript(jobId: string): Promise<StreamTranscript> {
  return readTranscript(await apiFetch(transcriptPath(jobId), { cache: 'no-store' }));
}

export async function transcribeStream(jobId: string, stretches: readonly Stretch[]): Promise<StreamTranscript> {
  const spans = stretches.map((stretch) => ({ start_seconds: stretch.start, end_seconds: stretch.end }));
  return readTranscript(await apiFetch(transcriptPath(jobId), jsonInit('POST', { spans })));
}

export async function patchStreamVoiceLine(target: { jobId: string; lineId: string }, patch: { text: string } | { discarded: boolean }): Promise<void> {
  await readAnswer(await apiFetch(`${transcriptPath(target.jobId)}/lines/${encodeURIComponent(target.lineId)}`, jsonInit('PATCH', patch)));
}

export type StreamSubtitlesPatch = { shorts: boolean } | { summary: boolean } | { video: string; line_id: string; hidden: boolean };

export async function patchStreamSubtitles(jobId: string, patch: StreamSubtitlesPatch): Promise<StreamSubtitles> {
  const subtitles = parseStreamSubtitles(await readAnswer(await apiFetch(`/api/streams/${encodeURIComponent(jobId)}/subtitles`, jsonInit('PATCH', patch))));
  if (subtitles === null) throw new Error('stream subtitles: unexpected answer');
  return subtitles;
}
