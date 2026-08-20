import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import {
  APP_UPDATE_STATE,
  AppUpdateController,
  GITHUB_LATEST_RELEASE_URL,
  checksumForFile,
  compareVersions,
  digestMatches,
  INSTALLER_SPAWN_ARGS,
  installerAssetName,
  parseGithubLatestRelease,
  parseReleaseVersion,
  releaseDownloadUrl,
  type AppUpdateHost,
  type AppUpdateStatus,
} from './app-update.ts';

test('parses and compares release versions', () => {
  const cases: Array<[string, string, string]> = [
    ['v2.4.29', '2.4.29', 'strips the leading v'],
    ['2.4.29', '2.4.29', 'accepts a bare semver tag'],
  ];
  for (const [input, want, name] of cases) {
    assert.equal(parseReleaseVersion(input), want, name);
  }

  for (const invalid of ['', 'latest', 'v2.4', '2.4.29-beta', 'v2.4.29.1', '../2.4.29']) {
    assert.throws(() => parseReleaseVersion(invalid), /unsupported release tag/);
  }

  const comparisons: Array<[string, string, number]> = [
    ['2.4.28', '2.4.29', -1],
    ['2.4.29', '2.4.29', 0],
    ['2.10.0', '2.9.9', 1],
    ['3.0.0', '2.99.99', 1],
  ];
  for (const [left, right, want] of comparisons) {
    assert.equal(compareVersions(left, right), want, `${left} vs ${right}`);
  }
});

test('silent NSIS apply asks electron-builder to relaunch after replace', () => {
  const cases: Array<{ flag: string; reason: string }> = [
    { flag: '/S', reason: 'silent so the wizard never appears' },
    { flag: '--updated', reason: 'NSIS treats this as a replace of a running install' },
    { flag: '--force-run', reason: 'assisted silent installs skip the finish-page Run checkbox' },
  ];
  assert.deepEqual([...INSTALLER_SPAWN_ARGS], cases.map((row) => row.flag));

  const desktopDirectory = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
  const manifest = JSON.parse(fs.readFileSync(path.join(desktopDirectory, 'package.json'), 'utf8'));
  assert.equal(manifest.build.nsis.include, 'build/installer.nsh');
  const nsis = fs.readFileSync(path.join(desktopDirectory, 'build', 'installer.nsh'), 'utf8').replaceAll('\r\n', '\n');
  const nsisLines = nsis.split('\n').map((line) => line.trim());
  assert.equal(
    nsisLines.some((line) => line.startsWith('!insertmacro StartApp')),
    false,
    'StartApp redeclares Var startAppArgs when installSection expands doStartApp',
  );
  const launch = '${StdUtils.ExecShellAsUser} $0 "$launchLink" "open" "--updated"';
  const required = [
    '!macro customInstall',
    '${if} ${isUpdated}',
    '${andIf} ${Silent}',
    '${ifNot} ${isForceRun}',
    'HideWindow',
    launch,
  ];
  for (const token of required) {
    assert.equal(nsis.includes(token), true, token);
  }
  const forceRunAt = nsis.indexOf('${ifNot} ${isForceRun}');
  const launchAt = nsis.indexOf(launch);
  const endIfAt = nsis.indexOf('${endIf}', forceRunAt);
  assert.notEqual(forceRunAt, -1);
  assert.ok(launchAt > forceRunAt && launchAt < endIfAt, 'ExecShellAsUser must sit inside ifNot isForceRun');

  const mainSource = fs.readFileSync(path.join(desktopDirectory, 'src', 'main.ts'), 'utf8');
  assert.equal(mainSource.includes('spawn(installerPath, [...INSTALLER_SPAWN_ARGS]'), true);
});

test('builds installer URLs only for the release contract', () => {
  assert.equal(installerAssetName('v2.4.29'), 'ClipHub.Studio.Setup.2.4.29.exe');
  assert.equal(
    releaseDownloadUrl('2.4.29', 'ClipHub.Studio.Setup.2.4.29.exe'),
    'https://github.com/rechedev9/cliphub/releases/download/v2.4.29/ClipHub.Studio.Setup.2.4.29.exe',
  );
  assert.equal(
    releaseDownloadUrl('2.4.29', 'SHA256SUMS.txt'),
    'https://github.com/rechedev9/cliphub/releases/download/v2.4.29/SHA256SUMS.txt',
  );
  assert.throws(
    () => releaseDownloadUrl('2.4.29', 'malware.exe'),
    /unexpected release asset/,
  );
});

test('landing download URL matches desktop version and the updater always hits GitHub latest', () => {
  const desktopDirectory = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
  const version = JSON.parse(fs.readFileSync(path.join(desktopDirectory, 'package.json'), 'utf8')).version;
  assert.equal(typeof version, 'string');
  const installer = installerAssetName(version);
  const downloadUrl = releaseDownloadUrl(version, installer);
  const landing = fs.readFileSync(path.join(desktopDirectory, '..', 'landing', 'app', 'page.tsx'), 'utf8');
  assert.equal(landing.includes(downloadUrl), true, downloadUrl);
  assert.equal(landing.includes(`const RELEASE_VERSION = "v${version}"`), true, version);
  assert.equal(
    GITHUB_LATEST_RELEASE_URL,
    'https://api.github.com/repos/rechedev9/cliphub/releases/latest',
  );
});

test('reads GitHub latest JSON and SHA256SUMS.txt entries', () => {
  assert.equal(
    parseGithubLatestRelease(JSON.stringify({ tag_name: 'v2.4.30', draft: false, prerelease: false })),
    '2.4.30',
  );
  for (const body of [
    '{',
    JSON.stringify({ tag_name: 'nightly' }),
    JSON.stringify({ tag_name: 'v2.4.30', prerelease: true }),
    JSON.stringify({ tag_name: 'v2.4.30', draft: true }),
    JSON.stringify({ name: 'v2.4.30' }),
  ]) {
    assert.throws(() => parseGithubLatestRelease(body));
  }

  const sums = [
    '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  ClipHub.Studio.Setup.2.4.30.exe',
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ClipHub.Studio.Setup.2.4.30.exe.blockmap',
  ].join('\n');
  assert.equal(
    checksumForFile(sums, 'ClipHub.Studio.Setup.2.4.30.exe'),
    '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
  );
  assert.throws(() => checksumForFile(sums, 'other.exe'), /missing/);
  assert.throws(() => checksumForFile('not a checksum', 'ClipHub.Studio.Setup.2.4.30.exe'));
  assert.equal(
    digestMatches(
      '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
      '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
    ),
    true,
  );
  assert.equal(
    digestMatches(
      '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
      '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdee',
    ),
    false,
  );
});

test('controller reports current, available, verified ready, and apply', async (t) => {
  const payload = Buffer.from('cliphub-installer-bytes');
  const digest = createHash('sha256').update(payload).digest('hex');
  const fake = makeHost(t, {
    currentVersion: '2.4.28',
    latest: '2.4.29',
    installer: payload,
    digest,
  });

  const controller = new AppUpdateController(fake.host);
  const seen: AppUpdateStatus['state'][] = [];
  controller.subscribe((status) => seen.push(status.state));

  await controller.check();
  assert.deepEqual(controller.status(), {
    state: APP_UPDATE_STATE.available,
    version: '2.4.29',
    currentVersion: '2.4.28',
  });

  await controller.install();
  assert.deepEqual(controller.status(), { state: APP_UPDATE_STATE.ready, version: '2.4.29' });
  assert.equal(fake.spawned.length, 0);
  assert.equal(fake.quit, 0);

  await controller.check({ quiet: true });
  assert.equal(controller.status().state, APP_UPDATE_STATE.ready);

  await controller.install();
  assert.equal(fake.spawned.length, 1);
  assert.match(fake.spawned[0] ?? '', /ClipHub\.Studio\.Setup\.2\.4\.29\.exe$/);
  assert.equal(fake.quit, 1);
  assert.ok(seen.includes(APP_UPDATE_STATE.downloading));
  assert.ok(seen.includes(APP_UPDATE_STATE.installing));
});

test('controller treats an equal or older latest tag as current', async (t) => {
  const fake = makeHost(t, {
    currentVersion: '2.4.29',
    latest: '2.4.29',
    installer: Buffer.from('unused'),
    digest: '00'.repeat(32),
  });
  const controller = new AppUpdateController(fake.host);
  await controller.check();
  assert.deepEqual(controller.status(), { state: APP_UPDATE_STATE.current, version: '2.4.29' });
});

test('controller stays idle on a quiet check failure and errors on a user check', async (t) => {
  const fake = makeHost(t, {
    currentVersion: '2.4.28',
    latest: '2.4.29',
    installer: Buffer.from('unused'),
    digest: '00'.repeat(32),
  });
  fake.host.fetchText = async () => {
    throw new Error('GET https://api.github.com/repos/rechedev9/cliphub/releases/latest: HTTP 503');
  };
  const controller = new AppUpdateController(fake.host);
  await controller.check({ quiet: true });
  assert.equal(controller.status().state, APP_UPDATE_STATE.idle);
  await controller.check();
  assert.equal(controller.status().state, APP_UPDATE_STATE.error);

  const unpackaged = new AppUpdateController({ ...fake.host, isPackaged: false });
  assert.equal(unpackaged.status().state, APP_UPDATE_STATE.unavailable);
  await unpackaged.check();
  assert.equal(unpackaged.status().state, APP_UPDATE_STATE.unavailable);
});

test('controller rejects a hash mismatch without leaving an installer', async (t) => {
  const payload = Buffer.from('cliphub-installer-bytes');
  const fake = makeHost(t, {
    currentVersion: '2.4.28',
    latest: '2.4.29',
    installer: payload,
    digest: 'ff'.repeat(32),
  });
  const controller = new AppUpdateController(fake.host);
  await controller.check();
  await controller.install();
  assert.equal(controller.status().state, APP_UPDATE_STATE.error);
  const destination = path.join(fake.host.updatesDirectory, 'ClipHub.Studio.Setup.2.4.29.exe');
  assert.equal(fs.existsSync(destination), false);
  assert.equal(fake.spawned.length, 0);
  assert.equal(fake.quit, 0);
});

function makeHost(
  t: { after: (fn: () => void) => void },
  input: { currentVersion: string; latest: string; installer: Buffer; digest: string },
): {
  host: AppUpdateHost;
  spawned: string[];
  quit: number;
} {
  const updatesDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-update-'));
  t.after(() => fs.rmSync(updatesDirectory, { recursive: true, force: true }));
  const spawned: string[] = [];
  const fake = {
    spawned,
    quit: 0,
    host: {
      currentVersion: input.currentVersion,
      isPackaged: true,
      platform: 'win32' as const,
      userAgent: `ClipHub-Studio/${input.currentVersion}`,
      updatesDirectory,
      fetchText: async (url: string): Promise<string> => {
        if (url === 'https://api.github.com/repos/rechedev9/cliphub/releases/latest') {
          return JSON.stringify({ tag_name: `v${input.latest}` });
        }
        if (url === `https://github.com/rechedev9/cliphub/releases/download/v${input.latest}/SHA256SUMS.txt`) {
          return `${input.digest}  ClipHub.Studio.Setup.${input.latest}.exe\n`;
        }
        throw new Error(`unexpected fetch ${url}`);
      },
      downloadFile: async (_url: string, destination: string): Promise<string> => {
        fs.mkdirSync(path.dirname(destination), { recursive: true });
        fs.writeFileSync(destination, input.installer);
        return createHash('sha256').update(input.installer).digest('hex');
      },
      spawnInstaller: async (installerPath: string): Promise<void> => {
        spawned.push(installerPath);
      },
      quitApp: (): void => {
        fake.quit += 1;
      },
      log: (): void => {},
    } satisfies AppUpdateHost,
  };
  return fake;
}
