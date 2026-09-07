/** The ClipHub Portal bridge settings `internal/cloudbridge` reads. */
export const BRIDGE_ENVIRONMENT_KEYS = ['ZV_BRIDGE_URL', 'ZV_BRIDGE_TOKEN'] as const;

/**
 * Forward the bridge settings into the orchestrator's curated environment.
 * The orchestrator rejects a half-configured bridge, so forward the pair only
 * when both are set: one alone would turn a launch into a startup error
 * instead of simply leaving the bridge off, which is what every install
 * without a portal wants.
 */
export function bridgeEnvironment(source: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {};
  for (const key of BRIDGE_ENVIRONMENT_KEYS) {
    const value = source[key];
    if (value === undefined || value.trim() === '') return {};
    env[key] = value;
  }
  return env;
}
