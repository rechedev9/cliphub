export type UpstreamSteamAccount = {
  steamId?: unknown;
  authCodeSet?: unknown;
  apiKeySet?: unknown;
  knownCode?: unknown;
  historyConfigured?: unknown;
  gcConfigured?: unknown;
  matches?: unknown;
};

type UpstreamMatch = {
  shareCode?: unknown;
  matchId?: unknown;
  discoveredAt?: unknown;
};

export function whitelistSteamAccount(data: UpstreamSteamAccount): Record<string, unknown> {
  const matches = Array.isArray(data.matches)
    ? data.matches.flatMap((item) => {
        const row = item as UpstreamMatch;
        if (typeof row.shareCode !== 'string' || typeof row.matchId !== 'string') return [];
        return [{
          shareCode: row.shareCode,
          matchId: row.matchId,
          discoveredAt: typeof row.discoveredAt === 'string' ? row.discoveredAt : undefined,
        }];
      })
    : [];
  return {
    steamId: typeof data.steamId === 'string' ? data.steamId : '',
    authCodeSet: data.authCodeSet === true,
    apiKeySet: data.apiKeySet === true,
    knownCode: typeof data.knownCode === 'string' ? data.knownCode : '',
    historyConfigured: data.historyConfigured === true,
    gcConfigured: data.gcConfigured === true,
    matches,
  };
}
