/** Copies an environment while dropping every casing variant of the credential. */
export function environmentWithoutXAIAPIKey(environment = process.env) {
  const sanitized = { ...environment };
  for (const name of Object.keys(sanitized)) {
    if (name.toLowerCase() === 'xai_api_key') delete sanitized[name];
  }
  return sanitized;
}

/** Makes the established unsigned release flow deterministic on developer machines. */
export function environmentWithoutCodeSigningCredentials(environment = process.env) {
  const sanitized = { ...environment };
  for (const name of Object.keys(sanitized)) {
    const normalized = name.toUpperCase();
    if (normalized === 'CSC_LINK'
      || normalized === 'CSC_KEY_PASSWORD'
      || normalized === 'WIN_CSC_LINK'
      || normalized === 'WIN_CSC_KEY_PASSWORD') {
      delete sanitized[name];
    }
  }
  sanitized.CSC_IDENTITY_AUTO_DISCOVERY = 'false';
  return sanitized;
}

// Release subprocesses get only host/runtime locations and deterministic build
// controls. API keys, mutation capabilities, proxy credentials, GitHub tokens,
// and arbitrary npm configuration never cross the release boundary.
const RELEASE_ENVIRONMENT_ALLOWLIST = new Set([
  'APPDATA',
  'CI',
  'COLORTERM',
  'COMSPEC',
  'ELECTRON_BUILDER_CACHE',
  'ELECTRON_CACHE',
  'FORCE_COLOR',
  'GOCACHE',
  'GOENV',
  'GOFLAGS',
  'GOMODCACHE',
  'GOPATH',
  'GOROOT',
  'HOME',
  'HOMEDRIVE',
  'HOMEPATH',
  'LANG',
  'LC_ALL',
  'LOCALAPPDATA',
  'NUMBER_OF_PROCESSORS',
  'OS',
  'PATH',
  'PATHEXT',
  'PROCESSOR_ARCHITECTURE',
  'PROCESSOR_IDENTIFIER',
  'PROGRAMDATA',
  'PROGRAMFILES',
  'PROGRAMFILES(X86)',
  'PROGRAMW6432',
  'SOURCE_DATE_EPOCH',
  'SYSTEMDRIVE',
  'SYSTEMROOT',
  'TEMP',
  'TERM',
  'TMP',
  'TMPDIR',
  'TZ',
  'USERDOMAIN',
  'USERNAME',
  'USERPROFILE',
  'WINDIR',
]);

export function releaseBuildEnvironment(environment = process.env) {
  const sanitized = {};
  for (const [name, value] of Object.entries(environment)) {
    if (value !== undefined && RELEASE_ENVIRONMENT_ALLOWLIST.has(name.toUpperCase())) {
      sanitized[name] = value;
    }
  }
  sanitized.CSC_IDENTITY_AUTO_DISCOVERY = 'false';
  return sanitized;
}
