const allowedExact = new Set([
  'PATH', 'Path', 'HOME', 'USERPROFILE', 'HOMEDRIVE', 'HOMEPATH',
  'TMPDIR', 'TMP', 'TEMP', 'SYSTEMROOT', 'SystemRoot', 'WINDIR', 'COMSPEC', 'ComSpec', 'PATHEXT',
  'LOCALAPPDATA', 'APPDATA', 'PROGRAMDATA', 'PROGRAMFILES', 'PROGRAMFILES(X86)',
  'LANG', 'LANGUAGE', 'TZ', 'TERM', 'CI',
  'GOROOT', 'GOPATH', 'GOMODCACHE', 'GOCACHE', 'GOENV', 'GOFLAGS', 'GOTOOLCHAIN',
  'CGO_ENABLED', 'CC', 'CXX',
  'PNPM_HOME', 'COREPACK_HOME', 'NPM_CONFIG_CACHE', 'PLAYWRIGHT_BROWSERS_PATH',
  'SSL_CERT_FILE', 'SSL_CERT_DIR', 'XDG_CACHE_HOME', 'XDG_RUNTIME_DIR',
  'DISPLAY', 'WAYLAND_DISPLAY',
]);

function allowedName(name) {
  return allowedExact.has(name) || name.startsWith('LC_');
}

export function captureLabEnvironment(base = process.env, overrides = {}) {
  const environment = {};
  for (const [name, value] of Object.entries(base)) {
    if (allowedName(name)) environment[name] = value;
  }
  Object.assign(environment, {
    NEXT_TELEMETRY_DISABLED: '1',
    DO_NOT_TRACK: '1',
  }, overrides);
  return environment;
}

export function scrubCaptureLabProcessEnvironment() {
  for (const name of Object.keys(process.env)) {
    if (!allowedName(name)) delete process.env[name];
  }
  process.env.NEXT_TELEMETRY_DISABLED = '1';
  process.env.DO_NOT_TRACK = '1';
}
