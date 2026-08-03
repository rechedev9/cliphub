export interface OrchestratorEnvironmentOptions {
  dataDir: string;
  httpAddress: string;
  musicDir: string;
  recorderPath: string;
  securityEnvironment: object;
  toolEnvironment: object;
}

/**
 * Builds the environment for the bundled orchestrator.
 *
 * The recorder path is deliberately assigned after every inherited runtime
 * tool value. A packaged Studio installation must use the recorder shipped
 * beside its orchestrator rather than a stale developer override from the
 * Windows user environment.
 */
export function createOrchestratorEnvironment(
  options: OrchestratorEnvironmentOptions,
): NodeJS.ProcessEnv {
  return {
    ZV_DATABASE_URL: 'sqlite',
    ZV_DATA_DIR: options.dataDir,
    ZV_HTTP_ADDR: options.httpAddress,
    ZV_MUSIC_DIR: options.musicDir,
    ...options.securityEnvironment,
    ...options.toolEnvironment,
    ZV_RECORDER_PATH: options.recorderPath,
  };
}
