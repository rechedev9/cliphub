/**
 * Display helpers for FACEIT player avatars.
 *
 * The orchestrator only proxies an avatar it has a URL for (it learns the URL
 * from the followed list or a profile lookup), so a player record without
 * `avatar` would always answer 404 and every such player cost one console
 * error per load. Callers gate the <img> on `hasFaceitAvatar` and fall back to
 * `playerInitial` otherwise.
 */

/** Whether the player record says there is an avatar the proxy can serve. */
export function hasFaceitAvatar(player: { avatar?: string }): boolean {
  return typeof player.avatar === 'string' && player.avatar.trim() !== '';
}

/** The proxied avatar route for one player. */
export function faceitAvatarSrc(playerID: string): string {
  return `/api/faceit/players/${encodeURIComponent(playerID)}/avatar`;
}

const LETTER_RE = /\p{L}/u;
const ALPHANUMERIC_RE = /[\p{L}\p{N}]/u;

/**
 * The single character shown when there is no avatar: the first letter of the
 * nickname, else its first digit, else "?". Nicknames like "-SYPHO" or
 * "1769-" otherwise rendered a bare punctuation mark or a number.
 */
export function playerInitial(nickname: string): string {
  const chars = Array.from(nickname);
  const pick = chars.find((char) => LETTER_RE.test(char)) ?? chars.find((char) => ALPHANUMERIC_RE.test(char));
  return pick === undefined ? '?' : pick.toLocaleUpperCase();
}
