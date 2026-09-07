// Keep the error chain useful before it leaves the machine. Do not attach
// complete logs, environment variables, demo metadata, or arbitrary attributes.
export const MAX_DIAGNOSTIC_BYTES = 2048;
const MAX_INPUT_CHARACTERS = 64 * 1024;

const REDACTIONS: Array<[RegExp, string]> = [
  // Bound individual tokens before the more specific redactors inspect them.
  [/[^\s]{256,}/g, '[long-token]'],
  [/\x1b\[[0-?]*[ -/]*[@-~]/g, ''],
  [/-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----/g, '[credential]'],
  [/\b(?:[a-z0-9]+[_-])*(?:authorization|proxy-authorization|password|passwd|token|access_token|refresh_token|api[_-]?key|ingest[_-]?key|secret|cookie|set-cookie)\s*["']?\s*[:=]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|(?:bearer|basic)\s+[^\s,;]+|[^\s,;]+)/gi, '[credential]'],
  [/\b(?:bearer|basic)\s+[^\s,;]+/gi, '[credential]'],
  [/\b(?:https?|ftp|steam):\/\/[^\s<>"']+/gi, '[url]'],
  [/"[a-z]:[\\/][^"\r\n]*"|'[a-z]:[\\/][^'\r\n]*'|[a-z]:[\\/][^:\r\n<>"',;]*/gi, '[path]'],
  [/"\\\\[^"\r\n]*"|'\\\\[^'\r\n]*'|\\\\[^\s<>"']+/g, '[path]'],
  [/"\/[^"\r\n]*"|'\/[^'\r\n]*'|\/[^\s<>"']+/g, '[path]'],
  [/[a-z0-9.!#$%&'*+\/=?^_`{|}~-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)+/gi, '[email]'],
  [/\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b/g, '[address]'],
  [/\b[0-9]{17}\b/g, '[steamid]'],
  [/[^\s\\/<>:"']+\.(?:dem|mp4|mkv|avi|webm|wav|mp3|m4a|flac)\b/gi, '[media]'],
  [/\b[a-z0-9_-]{40,}\b/gi, '[identifier]'],
];

export function diagnosticMessage(value: unknown): string {
  let text = '';
  if (value instanceof Error) text = errorChain(value);
  else if (typeof value === 'string') text = value;
  // Bound work while retaining the final subprocess failure in verbose output.
  if (text.length > MAX_INPUT_CHARACTERS) {
    text = `${text.slice(0, MAX_INPUT_CHARACTERS / 2)}\n[truncated]\n${text.slice(-MAX_INPUT_CHARACTERS / 2)}`;
  }
  for (const [pattern, replacement] of REDACTIONS) text = text.replace(pattern, replacement);
  text = text.replace(/[\x00-\x08\x0b-\x1f\x7f]/g, '').trim();
  if (Buffer.byteLength(text, 'utf8') <= MAX_DIAGNOSTIC_BYTES) return text;
  const marker = '\n[truncated]\n';
  const half = Math.floor((MAX_DIAGNOSTIC_BYTES - Buffer.byteLength(marker)) / 2);
  return `${fitBytes(text, half)}${marker}${fitBytes(text, half, true)}`;
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
