export const FACEIT_PLAYER_ID_RE = /^[A-Za-z0-9_-]{1,128}$/;

export type UpstreamFaceitPlayer = {
  id?: unknown;
  nickname?: unknown;
  avatar?: unknown;
  profile_url?: unknown;
  steam_id64?: unknown;
  country?: unknown;
  skill_level?: unknown;
  elo?: unknown;
  followed_at?: unknown;
};

export function whitelistFaceitPlayer(raw: UpstreamFaceitPlayer | undefined): Record<string, unknown> | null {
  if (raw === undefined || typeof raw.id !== 'string' || typeof raw.nickname !== 'string' || typeof raw.profile_url !== 'string') {
    return null;
  }
  const player: Record<string, unknown> = {
    id: raw.id,
    nickname: raw.nickname,
    profile_url: raw.profile_url,
  };
  if (typeof raw.avatar === 'string') player.avatar = raw.avatar;
  if (typeof raw.steam_id64 === 'string') player.steam_id64 = raw.steam_id64;
  if (typeof raw.country === 'string') player.country = raw.country;
  if (typeof raw.skill_level === 'number') player.skill_level = raw.skill_level;
  if (typeof raw.elo === 'number') player.elo = raw.elo;
  if (typeof raw.followed_at === 'string') player.followed_at = raw.followed_at;
  return player;
}
