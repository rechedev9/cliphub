/**
 * Settings of the orchestrator's cloud client. `ZV_CLOUD_URL` travels only when set (unset means
 * the default portal, `off` disables the client); the Studio version always does.
 */
export function cloudEnvironment(source: NodeJS.ProcessEnv, studioVersion: string): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { ZV_STUDIO_VERSION: studioVersion };
  const url = source.ZV_CLOUD_URL?.trim();
  if (url !== undefined && url !== '') env.ZV_CLOUD_URL = url;
  return env;
}
