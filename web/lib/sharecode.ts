// Shape only. The base-57 decode lives in internal/sharecode, not here.

export const SHARE_CODE_DICTIONARY = 'ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789';

const PAYLOAD_LENGTH = 25;

export type ShareCodeCheck =
  | { readonly ok: true; readonly normalized: string }
  | { readonly ok: false; readonly reason: 'empty' | 'length' | 'character'; readonly message: string };

export function normalizeShareCode(raw: string): string {
  return raw
    .trim()
    .replace(/\s+/g, '')
    .replace(/^csgo-/i, '')
    .replace(/-/g, '');
}

export function checkShareCode(raw: string): ShareCodeCheck {
  const normalized = normalizeShareCode(raw);
  if (normalized === '') {
    return { ok: false, reason: 'empty', message: 'Pega el código de la partida.' };
  }
  if (normalized.length !== PAYLOAD_LENGTH) {
    return {
      ok: false,
      reason: 'length',
      message: `Un código de partida tiene 25 caracteres tras los guiones; este tiene ${normalized.length}.`,
    };
  }
  for (const ch of normalized) {
    if (!SHARE_CODE_DICTIONARY.includes(ch)) {
      return {
        ok: false,
        reason: 'character',
        message: `El carácter «${ch}» no puede aparecer en un código de partida.`,
      };
    }
  }
  return { ok: true, normalized };
}
