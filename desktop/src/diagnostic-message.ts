// Keep the error chain useful before it leaves the machine. Do not attach
// complete logs, environment variables, demo metadata, or arbitrary attributes.
export const MAX_DIAGNOSTIC_BYTES = 2048;
const MAX_INPUT_CHARACTERS = 64 * 1024;

// The collector repeats this filter (internal/telemetry/diagnostic_message.go)
// and both run the shared fixtures. The rules run in three groups: secrets
// first, then URLs and paths (which a few known technical tokens are protected
// from), then identifiers.
const SECRET_REDACTIONS: Array<[RegExp, string]> = [
  // Bound individual tokens before the more specific redactors inspect them.
  [/[^\s]{256,}/g, '[long-token]'],
  [/\x1b\[[0-?]*[ -/]*[@-~]/g, ''],
  [/-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----/g, '[credential]'],
  [/\b(?:[a-z0-9]+[_-])*(?:authorization|proxy-authorization|password|passwd|token|access_token|refresh_token|api[_-]?key|ingest[_-]?key|secret|cookie|set-cookie)\s*["']?\s*[:=]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|(?:bearer|basic)\s+[^\s,;]+|[^\s,;]+)/gi, '[credential]'],
  [/\b(?:bearer|basic)\s+[^\s,;]+/gi, '[credential]'],
];

const PATH_REDACTIONS: Array<[RegExp, string]> = [
  [/\b(?:https?|ftp|steam):\/\/[^\s<>"']+/gi, '[url]'],
  [/"[a-z]:[\\/][^"\r\n]*"|'[a-z]:[\\/][^'\r\n]*'|[a-z]:[\\/][^:\r\n<>"',;]*/gi, '[path]'],
  [/"\\\\[^"\r\n]*"|'\\\\[^'\r\n]*'|\\\\[^\s<>"']+/g, '[path]'],
  [/"\/[^"\r\n]*"|'\/[^'\r\n]*'|\/[^\s<>"']+/g, '[path]'],
];

const IDENTIFIER_REDACTIONS: Array<[RegExp, string]> = [
  [/[a-z0-9.!#$%&'*+\/=?^_`{|}~-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)+/gi, '[email]'],
  [/\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b/g, '[address]'],
  [/\b[0-9]{17}\b/g, '[steamid]'],
  [/[^\s\\/<>:"']+\.(?:dem|mp4|mkv|avi|webm|wav|mp3|m4a|flac)\b/gi, '[media]'],
  [/\b[a-z0-9_-]{40,}\b/gi, '[identifier]'],
];

// Protected tokens are swapped for private-use placeholders before the URL and
// path rules and restored right after them. Only tokens that the identifier
// rules would leave unchanged qualify, so the filter stays idempotent.
const PLACEHOLDER_BASE = 0xe000;
const MAX_PLACEHOLDERS = 0xf8ff - PLACEHOLDER_BASE + 1;
const PLACEHOLDERS = /[-]/g;
// The Studio web UI is served from loopback; its stack frames keep the bundle
// location but not the port.
const PROTECTED_APP_ASSET = /(?:http:\/\/127\.0\.0\.1:[0-9]{1,5}|\[app\])\/_next\/static\/chunks\/[A-Za-z0-9_.\-()\[\]@~/]+\.js:[0-9]{1,7}:[0-9]{1,7}/g;
const PROTECTED_WHITESPACE = /[^\t\n\f\r ]+/g;
const PROTECTED_ROUTE = /^\/api\/[A-Za-z0-9{}_.\-/]+$/;
// This repository's Go module path (go.mod): -trimpath panic frames start with it.
const PROTECTED_MODULE_FRAME = /\bgithub\.com\/rechedev9\/cliphub\/[A-Za-z0-9_./\-]+/g;
const PROTECTED_DATE = /\b[0-9]{4}\/[0-9]{2}\/[0-9]{2}\b/g;
const PROTECTED_FRACTION = /\b[0-9]{1,6}\/[0-9]{1,6}\b/g;
const UNSAFE_PATH_SEGMENT = /(?:^|\/)\.{1,2}(?:\/|$)|\/\//;

export function diagnosticMessage(value: unknown): string {
	return filterMessage(value, MAX_DIAGNOSTIC_BYTES);
}

/** Filter a technical log before chunking it; do not truncate it to an error summary. */
export function diagnosticLogMessage(value: unknown): string {
  if (value instanceof Error) {
    const stack = typeof value.stack === 'string' ? value.stack : '';
    return filterMessage(`${errorChain(value)}${stack ? `\n${stack}` : ''}`, 256 * 1024);
  }
  return filterMessage(value, 256 * 1024);
}

function filterMessage(value: unknown, limit: number): string {
  let text = '';
  if (value instanceof Error) text = errorChain(value);
  else if (typeof value === 'string') text = value;
  // Bound work while retaining the final subprocess failure in verbose output.
  if (text.length > MAX_INPUT_CHARACTERS) {
    text = `${text.slice(0, MAX_INPUT_CHARACTERS / 2)}\n[truncated]\n${text.slice(-MAX_INPUT_CHARACTERS / 2)}`;
  }
  text = text.replace(PLACEHOLDERS, '');
  text = applyRedactions(text, SECRET_REDACTIONS);
  const protectedTokens: string[] = [];
  text = protectTokens(text, protectedTokens);
  text = applyRedactions(text, PATH_REDACTIONS);
  text = text.replace(PLACEHOLDERS, (placeholder) => protectedTokens[placeholder.charCodeAt(0) - PLACEHOLDER_BASE] ?? '');
  text = applyRedactions(text, IDENTIFIER_REDACTIONS);
  text = text.replace(/[\x00-\x08\x0b-\x1f\x7f]/g, '').trim();
  if (Buffer.byteLength(text, 'utf8') <= limit) return text;
  const marker = '\n[truncated]\n';
  const half = Math.floor((limit - Buffer.byteLength(marker)) / 2);
  return `${fitBytes(text, half)}${marker}${fitBytes(text, half, true)}`;
}

function applyRedactions(text: string, rules: Array<[RegExp, string]>): string {
  for (const [pattern, replacement] of rules) text = text.replace(pattern, replacement);
  return text;
}

/**
 * Keep API route patterns, this module's panic frames, dates, fractions and
 * Studio bundle frames out of the path rules, which would otherwise turn them
 * into [path]. Anything else behaves exactly as before.
 */
function protectTokens(text: string, protectedTokens: string[]): string {
  const keep = (candidate: string, fallback: string): string => {
    if (
      protectedTokens.length >= MAX_PLACEHOLDERS ||
      UNSAFE_PATH_SEGMENT.test(candidate) ||
      applyRedactions(candidate, IDENTIFIER_REDACTIONS) !== candidate
    ) {
      return fallback;
    }
    protectedTokens.push(candidate);
    return String.fromCharCode(PLACEHOLDER_BASE + protectedTokens.length - 1);
  };
  text = text.replace(PROTECTED_APP_ASSET, (match) => keep(`[app]${match.slice(match.indexOf('/_next/'))}`, match));
  text = text.replace(PROTECTED_WHITESPACE, (token) => (PROTECTED_ROUTE.test(token) ? keep(token, token) : token));
  for (const pattern of [PROTECTED_MODULE_FRAME, PROTECTED_DATE, PROTECTED_FRACTION]) {
    text = text.replace(pattern, (match) => keep(match, match));
  }
  return text;
}

function errorChain(error: Error): string {
  const seen = new Set<Error>();
  const parts: string[] = [];
  let current: unknown = error;
  while (current instanceof Error && !seen.has(current) && parts.length < 4) {
    seen.add(current);
    const code = 'code' in current && typeof current.code === 'string' ? ` (${current.code})` : '';
    parts.push(`${current.name}${code}: ${current.message}`);
    current = current.cause;
  }
  return parts.join('\nCaused by: ');
}

function fitBytes(text: string, budget: number, tail = false): string {
  const characters = Array.from(text);
  if (tail) characters.reverse();
  const kept: string[] = [];
  let bytes = 0;
  for (const character of characters) {
    bytes += Buffer.byteLength(character, 'utf8');
    if (bytes > budget) break;
    kept.push(character);
  }
  if (tail) kept.reverse();
  return kept.join('');
}
