import { isServiceUnavailable, nonVideoExtension, STREAM_INVALID_URL_MESSAGE, STREAM_OFFLINE_MESSAGE } from './plan.ts';

/** Stream import validation and error copy for the /streams source form. */

export const STREAM_URL_REQUIRED_MESSAGE =
  'Pega una URL de clip o VOD de Twitch, YouTube o Kick. Para un archivo local, usa un MP4.';

export const STREAM_IMPORT_URL_FAIL_MESSAGE =
  'No se pudo importar el vídeo. Revisa el enlace e inténtalo de nuevo.';

export const STREAM_IMPORT_FILE_FAIL_MESSAGE = 'No se pudo procesar ese archivo. Prueba con otro MP4.';

export const STREAM_IMPORT_YTDLP_MISSING_MESSAGE =
  'Este equipo no puede descargar vídeos desde un enlace. Usa “Subir un MP4” o reinstala ClipHub Studio.';

export const STREAM_IMPORT_FILE_TOO_LARGE_MESSAGE =
  'Ese archivo supera el límite de 2 GB. Recorta la grabación o importa el clip desde su enlace.';

export const STREAM_IMPORT_FILE_UNREADABLE_MESSAGE =
  'Ese archivo no se puede leer como vídeo. Prueba con otro MP4.';

/**
 * Exact copy of `allowedProviderHosts` in internal/vodfetch/vodfetch.go. The
 * server stays the authority (it also checks each provider's path shape and
 * answers `invalid_source_url`); this list only stops a request that is certain
 * to be rejected.
 */
export const STREAM_SOURCE_HOSTS: ReadonlySet<string> = new Set([
  'clips.twitch.tv',
  'm.twitch.tv',
  'twitch.tv',
  'www.twitch.tv',
  'm.youtube.com',
  'music.youtube.com',
  'www.youtube.com',
  'youtu.be',
  'youtube.com',
  'kick.com',
  'www.kick.com',
]);

/**
 * Spanish validation message for a pasted source URL, or null when it may be
 * sent. Mirrors `vodfetch.ValidateSource`: HTTPS only, an allowed provider
 * host, no credentials and no explicit port.
 */
export function streamSourceUrlError(raw: string): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return STREAM_URL_REQUIRED_MESSAGE;
  const badExt = nonVideoExtension(trimmed);
  if (badExt) {
    return `Esa URL apunta a un archivo .${badExt}, no a un vídeo. Pega el enlace de un clip o VOD de Twitch, YouTube o Kick, o usa “Subir un MP4”.`;
  }
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return STREAM_INVALID_URL_MESSAGE;
  }
  if (url.protocol !== 'https:' || url.username || url.password || url.port) return STREAM_INVALID_URL_MESSAGE;
  if (!STREAM_SOURCE_HOSTS.has(url.hostname.toLowerCase())) return STREAM_INVALID_URL_MESSAGE;
  return null;
}

type ImportErrorRule = { match: (message: string, status?: number) => boolean; message: string };

/**
 * Known English bodies from `POST /api/stream-jobs` and the Next proxy
 * (`app/api/streams/route.ts`, `lib/api/bounded-request-body.ts`). Anything
 * else falls back to the generic copy so raw server text never reaches the form.
 */
const URL_IMPORT_RULES: readonly ImportErrorRule[] = [
  {
    match: (message) => message.startsWith('acquiring a stream job by URL is not configured'),
    message: STREAM_IMPORT_YTDLP_MISSING_MESSAGE,
  },
  { match: (message) => message.startsWith('invalid source_url'), message: STREAM_INVALID_URL_MESSAGE },
];

const FILE_IMPORT_RULES: readonly ImportErrorRule[] = [
  {
    match: (message, status) =>
      status === 413 || message === 'file too large' || message === 'request body too large',
    message: STREAM_IMPORT_FILE_TOO_LARGE_MESSAGE,
  },
  { match: (message) => message.startsWith('probe video:'), message: STREAM_IMPORT_FILE_UNREADABLE_MESSAGE },
];

/** Spanish copy for a failed stream import; never the raw server body. */
export function streamImportErrorMessage(err: unknown, source: 'url' | 'file'): string {
  if (isServiceUnavailable(err)) return STREAM_OFFLINE_MESSAGE;
  const { code, status } = (err ?? {}) as { code?: unknown; status?: unknown };
  if (source === 'url' && code === 'invalid_source_url') return STREAM_INVALID_URL_MESSAGE;
  const message = err instanceof Error ? err.message : '';
  const httpStatus = typeof status === 'number' ? status : undefined;
  const rules = source === 'url' ? URL_IMPORT_RULES : FILE_IMPORT_RULES;
  const rule = rules.find((entry) => entry.match(message, httpStatus));
  if (rule) return rule.message;
  return source === 'url' ? STREAM_IMPORT_URL_FAIL_MESSAGE : STREAM_IMPORT_FILE_FAIL_MESSAGE;
}
