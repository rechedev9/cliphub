import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import {
  collectDeviceContext,
  diskBucket,
  gpuVendor,
  hasAACMediaFoundation,
  normalizeCPUModel,
  parseEditionID,
  ramBucket,
  toolVersion,
  versionValue,
  type DeviceContextOptions,
} from './device-context.ts';
import { diagnosticLogMessage, diagnosticMessage } from './diagnostic-message.ts';

const GIB = 2 ** 30;
const ENCODERS = [
  'Encoders:',
  ' V....D libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)',
  ' A....D aac                  AAC (Advanced Audio Coding)',
  ' A..... aac_mf               AAC via MediaFoundation (codec aac)',
].join('\n');

function tempDir(t: test.TestContext): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-device-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

function stubOptions(dir: string, overrides: Partial<DeviceContextOptions> = {}): DeviceContextOptions & { commands: string[] } {
  const commands: string[] = [];
  const toolsDir = path.join(dir, 'tools');
  return {
    commands,
    dataDir: path.join(dir, 'data'),
    toolsDir,
    ffmpegPath: path.join(toolsDir, 'ffmpeg', 'n8.1.2-30-g45f1910444-20260723', 'ffmpeg-n8.1-latest-win64-gpl-shared-8.1', 'bin', 'ffmpeg.exe'),
    hlaePath: path.join(toolsDir, 'hlae', '2.192.3', 'HLAE.exe'),
    cachePath: path.join(dir, 'device-context-cache.json'),
    appVersion: '5.2.1',
    platform: 'win32',
    systemVersion: () => '10.0.26200',
    gpuInfo: async () => ({
      gpuDevice: [
        { vendorId: 0x8086, deviceId: 0x4680, active: false, driverVersion: '31.0.101.4502' },
        { vendorId: 0x10de, deviceId: 0x2684, active: true, driverVersion: '32.0.15.6094' },
      ],
    }),
    cpus: () => Array.from({ length: 16 }, () => ({ model: 'AMD Ryzen 7 5800X3D 8-Core Processor            ' })),
    totalMemoryBytes: () => 31.9 * GIB,
    statfs: async (target) => {
      if (target === path.resolve(dir, 'data')) throw Object.assign(new Error('missing'), { code: 'ENOENT' });
      return { bavail: 120 * 1024, bsize: 1024 * 1024 };
    },
    runCommand: async (file, args) => {
      commands.push([path.basename(file), ...args].join(' '));
      if (file === 'reg') return '\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\r\n    EditionID    REG_SZ    ProfessionalN\r\n\r\n';
      return ENCODERS;
    },
    ...overrides,
  };
}

test('collects one bucketed line in the contract order', async (t) => {
  const options = stubOptions(tempDir(t));
  const line = await collectDeviceContext(options);
  assert.equal(line, [
    'win_build=v10.0.26200', 'edition=ProfessionalN', 'gpu_vendor=nvidia', 'gpu_device=0x2684',
    'gpu_driver=v32.0.15.6094', 'cpu_cores=16', 'cpu_model=AMD_Ryzen_7_5800X3D_8-Core_Processor',
    'ram_gb=32', 'disk_free_gb=50-200', 'ffmpeg=v8.1.2', 'aac_mf=yes', 'hlae=v2.192.3',
  ].join(' '));
});

test('the line passes both diagnostic filters unchanged', async (t) => {
  const line = await collectDeviceContext(stubOptions(tempDir(t), {
    cpus: () => [{ model: '13th Gen Intel(R) Core(TM) i9-13900K @ 3.00GHz' }, { model: 'x' }],
    gpuInfo: async () => ({ gpuDevice: [{ vendorId: 0x1002, deviceId: 0x744c, active: true, driverVersion: '31.0.24033.1003' }] }),
    hlaePath: path.join(tempDir(t), 'unrelated', 'HLAE.exe'),
  }));
  assert.equal(diagnosticLogMessage(line), line);
  assert.equal(diagnosticMessage(line), line);
  assert.doesNotMatch(line, /\[(?:address|path|identifier|email|media|url)\]/);
  assert.match(line, /gpu_driver=v31\.0\.24033\.1003/);
  assert.match(line, /hlae=unknown/);
});

test('the encoder probe runs once per install version', async (t) => {
  const dir = tempDir(t);
  const first = stubOptions(dir);
  await collectDeviceContext(first);
  assert.equal(first.commands.filter((command) => command.includes('-encoders')).length, 1);

  const second = stubOptions(dir);
  assert.match(await collectDeviceContext(second), /aac_mf=yes/);
  assert.equal(second.commands.filter((command) => command.includes('-encoders')).length, 0);

  const upgraded = stubOptions(dir, { appVersion: '5.3.0', runCommand: async (file) => (file === 'reg' ? '' : ' A....D aac   AAC') });
  assert.match(await collectDeviceContext(upgraded), /aac_mf=no/);
});

test('failed probes degrade to unknown or none without throwing', async (t) => {
  const line = await collectDeviceContext(stubOptions(tempDir(t), {
    ffmpegPath: undefined,
    hlaePath: undefined,
    systemVersion: () => { throw new Error('unavailable'); },
    gpuInfo: async () => { throw new Error('gpu process gone'); },
    cpus: () => [],
    totalMemoryBytes: () => Number.NaN,
    statfs: async () => { throw new Error('unsupported'); },
    runCommand: async () => { throw new Error('timed out'); },
  }));
  assert.equal(line, 'win_build=unknown edition=unknown gpu_vendor=unknown gpu_device=unknown gpu_driver=unknown '
    + 'cpu_cores=unknown cpu_model=unknown ram_gb=unknown disk_free_gb=unknown ffmpeg=none aac_mf=unknown hlae=none');
});

test('edition is only probed on Windows', async (t) => {
  const options = stubOptions(tempDir(t), { platform: 'linux' });
  assert.match(await collectDeviceContext(options), /edition=unknown/);
  assert.equal(options.commands.some((command) => command.startsWith('reg')), false);
});

test('maps PCI vendor ids', () => {
  assert.equal(gpuVendor(0x10de), 'nvidia');
  assert.equal(gpuVendor(0x1002), 'amd');
  assert.equal(gpuVendor(0x1022), 'amd');
  assert.equal(gpuVendor(0x8086), 'intel');
  assert.equal(gpuVendor(0x1414), 'other');
  assert.equal(gpuVendor(0), 'unknown');
  assert.equal(gpuVendor('0x10de'), 'unknown');
});

test('buckets memory and free disk space', () => {
  assert.equal(ramBucket(3.8 * GIB), '4');
  assert.equal(ramBucket(7.9 * GIB), '8');
  assert.equal(ramBucket(12 * GIB), '8');
  assert.equal(ramBucket(15.8 * GIB), '16');
  assert.equal(ramBucket(31.9 * GIB), '32');
  assert.equal(ramBucket(63.8 * GIB), '64+');
  assert.equal(ramBucket(128 * GIB), '64+');
  assert.equal(ramBucket(0), 'unknown');
  assert.equal(diskBucket(4 * GIB), '<10');
  assert.equal(diskBucket(10 * GIB), '10-50');
  assert.equal(diskBucket(120 * GIB), '50-200');
  assert.equal(diskBucket(900 * GIB), '200+');
  assert.equal(diskBucket(Number.NaN), 'unknown');
});

test('normalizes CPU models without identifiers or symbols', () => {
  assert.equal(normalizeCPUModel('Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz'), 'Intel_Core_i7-10700K_CPU_3.80GHz');
  assert.equal(normalizeCPUModel('  '), 'unknown');
  const long = normalizeCPUModel('AMD Ryzen Threadripper PRO 3995WX 64-Cores Workstation Edition');
  assert.ok(long.length <= 48);
  assert.equal(diagnosticLogMessage(`cpu_model=${long}`), `cpu_model=${long}`);
});

test('versions carry a v prefix and keep named pin suffixes', () => {
  assert.equal(versionValue('10.0.26200'), 'v10.0.26200');
  assert.equal(versionValue('n8.1.2-30-g45f1910444-20260723'), 'v8.1.2');
  assert.equal(versionValue('2.192.2-cliphub.1'), 'v2.192.2-cliphub.1');
  assert.equal(versionValue('32.0.15.6094'), 'v32.0.15.6094');
  assert.equal(versionValue('latest'), 'unknown');
  const tools = path.join('C:', 'data', 'tools');
  assert.equal(toolVersion(tools, 'hlae', path.join(tools, 'hlae', '2.192.3', 'HLAE.exe')), 'v2.192.3');
  assert.equal(toolVersion(tools, 'hlae', path.join('C:', 'elsewhere', 'HLAE.exe')), 'unknown');
  assert.equal(toolVersion(tools, 'hlae', path.join(tools, 'hlae', 'HLAE.exe')), 'unknown');
});

test('parses the registry edition and the encoder listing', () => {
  assert.equal(parseEditionID('    EditionID    REG_SZ    Core\r\n'), 'Core');
  assert.equal(parseEditionID('ERROR: The system was unable to find the specified registry key or value.'), 'unknown');
  assert.equal(hasAACMediaFoundation(ENCODERS), true);
  assert.equal(hasAACMediaFoundation(' A....D aac                  AAC (Advanced Audio Coding)'), false);
});
