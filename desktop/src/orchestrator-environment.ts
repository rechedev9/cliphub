export interface OrchestratorEnvironmentOptions {
  dataDir: string;
  httpAddress: string;
  musicDir: string;
  recorderPath: string;
  securityEnvironment: object;
  toolEnvironment: object;
  /** User-supplied Steam credentials; absent when none are set. */
  steamEnvironment?: object;
}

/** Bundled orchestrator env. Recorder path wins over a stale developer override. */
export function createOrchestratorEnvironment(
  options: OrchestratorEnvironmentOptions,
): NodeJS.ProcessEnv {
  return {
    ZV_DATABASE_URL: 'sqlite',
    ZV_DATA_DIR: options.dataDir,
    ZV_HTTP_ADDR: options.httpAddress,
    ZV_MUSIC_DIR: options.musicDir,
    GOLANG_PROTOBUF_REGISTRATION_CONFLICT: 'ignore',
    ...options.securityEnvironment,
    ...options.toolEnvironment,
    // Credentials before the recorder pin: only the user can supply them.
    ...(options.steamEnvironment ?? {}),
    ZV_RECORDER_PATH: options.recorderPath,
  };
}
