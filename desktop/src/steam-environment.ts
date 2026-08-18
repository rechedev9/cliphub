/** The Steam credentials `internal/steamresolve` reads, in the order it reads them. */
export const STEAM_ENVIRONMENT_KEYS = [
  'ZV_STEAM_USERNAME',
  'ZV_STEAM_PASSWORD',
  'ZV_STEAM_GUARD',
] as const;

/** Forward non-empty Steam env vars; a closed orchestrator env would drop them. */
export function steamEnvironment(source: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {};
  for (const key of STEAM_ENVIRONMENT_KEYS) {
    const value = source[key];
    if (value !== undefined && value.trim() !== '') env[key] = value;
  }
  return env;
}
