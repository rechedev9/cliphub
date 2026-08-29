#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { readFile, writeFile, mkdir, readdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const boundaryDirectories = [
  'cmd/zv',
  'cmd/zv-recorder',
  'internal/artifacts',
  'internal/capturetools',
  'internal/killplan',
  'internal/pathguard',
  'internal/recording',
];

async function boundaryFileList() {
  const files = [
    'go.mod',
    'scripts/build.ps1',
    'scripts/capture-lab-real-canary.ps1',
  ];
  for (const directory of boundaryDirectories) {
    for (const entry of await readdir(resolve(repoRoot, directory), { withFileTypes: true })) {
      if (entry.isFile() && entry.name.endsWith('.go') && !entry.name.endsWith('_test.go')) {
        files.push(`${directory}/${entry.name}`);
      }
    }
  }
  return files.sort();
}

function argsObject(args) {
  const command = args.shift();
  const options = {};
  for (let index = 0; index < args.length; index++) {
    const flag = args[index];
    if (!flag.startsWith('--')) throw new Error(`unexpected argument ${flag}`);
    const value = args[++index];
    if (!value || value.startsWith('--')) throw new Error(`${flag} requires a value`);
    options[flag.slice(2).replaceAll('-', '_')] = value;
  }
  return { command, options };
}

async function hashFile(path) {
  return createHash('sha256').update(await readFile(path)).digest('hex');
}

async function boundaryFingerprint() {
  const hash = createHash('sha256');
  const files = [];
  for (const path of await boundaryFileList()) {
    const absolute = resolve(repoRoot, path);
    const sha256 = await hashFile(absolute);
    hash.update(path.replaceAll('\\', '/')).update('\0').update(sha256).update('\n');
    files.push({ path, sha256 });
  }
  return { sha256: hash.digest('hex'), files };
}

function gitRevision() {
  try {
    return execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repoRoot, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function argvFlag(argv, name) {
  for (let index = 2; index < argv.length; index++) {
    if (argv[index] === name) return argv[index + 1];
    if (argv[index].startsWith(`${name}=`)) return argv[index].slice(name.length + 1);
  }
  return undefined;
}

function sameLocalPath(left, right) {
  const normalize = (value) => {
    const normalized = resolve(value);
    return process.platform === 'win32' ? normalized.toLowerCase() : normalized;
  };
  return normalize(left) === normalize(right);
}

async function writeJSON(path, value) {
  if (!path) return;
  await mkdir(dirname(resolve(path)), { recursive: true });
  await writeFile(path, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

async function fingerprint(options) {
  const boundary = await boundaryFingerprint();
  const report = {
    schema_version: 1,
    ok: true,
    repository_revision: gitRevision(),
    boundary,
    script: null,
  };
  if (options.script) {
    report.script = { path: resolve(options.script), sha256: await hashFile(options.script) };
  }
  await writeJSON(options.out, report);
  return report;
}

async function issue(options) {
  for (const required of ['recording_result', 'hlae_version', 'cs2_build', 'argv_json', 'out']) {
    if (!options[required]) throw new Error(`issue requires --${required.replaceAll('_', '-')}`);
  }
  const resultPath = resolve(options.recording_result);
  const result = JSON.parse(await readFile(resultPath, 'utf8'));
  if (result.capture_mode !== 'real' || result.capture_verified !== true || result.error || result.publication_pending) {
    throw new Error('certificate requires a successful capture_mode=real result with completed POV verification');
  }
  if (!/^[0-9a-f]{64}$/.test(result.capture_input_fingerprint ?? '')) {
    throw new Error('recording result lacks a valid capture input fingerprint');
  }
  const scriptPath = resolve(result.script);
  const argv = JSON.parse(await readFile(resolve(options.argv_json), 'utf8'));
  if (!Array.isArray(argv) || argv.length < 2 || argv.some((value) => typeof value !== 'string')) {
    throw new Error('--argv-json must name a JSON array containing executable and exact arguments');
  }
  if (argv[1] !== 'record') throw new Error('certified argv must invoke the unified zv record command');
  const outDir = resolve(dirname(resultPath));
  const demoPath = resolve(result.plan?.demo_path ?? '');
  const killPlanPath = resolve(argvFlag(argv, '--killplan') ?? '');
  const hlaePath = resolve(argvFlag(argv, '--hlae') ?? '');
  const cs2Path = resolve(argvFlag(argv, '--cs2') ?? '');
  if (!sameLocalPath(argvFlag(argv, '--demo') ?? '', demoPath) ||
      !sameLocalPath(argvFlag(argv, '--out') ?? '', outDir)) {
    throw new Error('certified argv --demo/--out do not match the validated recording result');
  }
  try {
    execFileSync('go', [
      'run', './scripts/capturelab/validate-real-result.go',
      '--result', resultPath, '--killplan', killPlanPath,
    ], { cwd: repoRoot, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
  } catch (error) {
    throw new Error(`Go recording validators rejected canary evidence: ${String(error.stderr ?? error.message).trim()}`);
  }
  const killPlan = JSON.parse(await readFile(killPlanPath, 'utf8'));
  const planSegmentIDs = (result.plan?.segments ?? []).map((segment) => segment.id).join(',');
  if (killPlan.demo?.sha256 !== result.plan?.demo_sha256 ||
      killPlan.target?.steamid64 !== result.plan?.target_steamid64 ||
      (killPlan.segments ?? []).map((segment) => segment.id).join(',') !== planSegmentIDs) {
    throw new Error('certified argv kill plan does not match the validated recording result');
  }
  const demoHash = await hashFile(demoPath);
  if (demoHash !== result.plan?.demo_sha256) throw new Error('demo SHA-256 does not match the validated recording plan');
  const artifacts = [];
  for (const artifact of result.artifacts ?? []) {
    if (artifact.role !== 'segment' || artifact.type !== 'video') continue;
    const path = resolve(artifact.path);
    artifacts.push({ segment_id: artifact.segment_id, path, sha256: await hashFile(path) });
  }
  if (artifacts.length === 0) throw new Error('validated canary contains no segment video artifacts');
  const boundary = await boundaryFingerprint();
  const certificate = {
    schema_version: 1,
    kind: 'cliphub-real-capture-compatibility-certificate',
    issued_at: new Date().toISOString(),
    repository_revision: gitRevision(),
    hlae_version: options.hlae_version,
    cs2_build: options.cs2_build,
    exact_argv: argv,
    executables: {
      zv: { path: resolve(argv[0]), sha256: await hashFile(resolve(argv[0])) },
      hlae: { path: hlaePath, sha256: await hashFile(hlaePath) },
      cs2: { path: cs2Path, sha256: await hashFile(cs2Path) },
    },
    kill_plan: { path: killPlanPath, sha256: await hashFile(killPlanPath) },
    capture_contract: result.plan?.capture_contract,
    capture_input_fingerprint: result.capture_input_fingerprint,
    recording_result: {
      path: resultPath,
      sha256: await hashFile(resultPath),
    },
    generated_script: {
      path: scriptPath,
      sha256: await hashFile(scriptPath),
    },
    demo: {
      path: demoPath,
      sha256: demoHash,
    },
    artifacts,
    validation: 'recording.ValidateRecordingAttempt+ValidateUploadResult',
    boundary,
    limitations: [
      'This certificate is local evidence, not a validation override.',
      'It becomes stale when a capture-boundary source, HLAE version, CS2 build, generated script, demo, or plan fingerprint changes.',
    ],
  };
  await writeJSON(options.out, certificate);
  return { schema_version: 1, ok: true, certificate: resolve(options.out), issued_at: certificate.issued_at };
}

async function check(options) {
  if (!options.certificate || !options.hlae_version || !options.cs2_build) {
    throw new Error('check requires --certificate, --hlae-version, and --cs2-build');
  }
  const certificate = JSON.parse(await readFile(options.certificate, 'utf8'));
  const boundary = await boundaryFingerprint();
  const stale_reasons = [];
  if (certificate.kind !== 'cliphub-real-capture-compatibility-certificate') stale_reasons.push('certificate kind is invalid');
  if (certificate.boundary?.sha256 !== boundary.sha256) stale_reasons.push('capture-boundary source fingerprint changed');
  if (options.hlae_version !== certificate.hlae_version) stale_reasons.push('HLAE version changed');
  if (options.cs2_build !== certificate.cs2_build) stale_reasons.push('CS2 build changed');
  const resultPath = resolve(options.recording_result ?? certificate.recording_result?.path ?? '');
  const scriptPath = resolve(options.script ?? certificate.generated_script?.path ?? '');
  try {
    if (await hashFile(resultPath) !== certificate.recording_result?.sha256) stale_reasons.push('recording result changed');
  } catch {
    stale_reasons.push('recording result is unavailable');
  }
  try {
    if (await hashFile(scriptPath) !== certificate.generated_script?.sha256) stale_reasons.push('generated recording.js changed');
  } catch {
    stale_reasons.push('generated recording.js is unavailable');
  }
  for (const [name, executable] of Object.entries(certificate.executables ?? {})) {
    try {
      if (await hashFile(executable.path) !== executable.sha256) stale_reasons.push(`${name} executable changed`);
    } catch {
      stale_reasons.push(`${name} executable is unavailable`);
    }
  }
  try {
    if (await hashFile(certificate.kill_plan?.path ?? '') !== certificate.kill_plan?.sha256) stale_reasons.push('kill plan changed');
  } catch {
    stale_reasons.push('kill plan is unavailable');
  }
  try {
    if (await hashFile(certificate.demo?.path ?? '') !== certificate.demo?.sha256) stale_reasons.push('demo changed');
  } catch {
    stale_reasons.push('demo is unavailable');
  }
  for (const artifact of certificate.artifacts ?? []) {
    try {
      if (await hashFile(artifact.path) !== artifact.sha256) stale_reasons.push(`segment artifact changed: ${artifact.segment_id}`);
    } catch {
      stale_reasons.push(`segment artifact is unavailable: ${artifact.segment_id}`);
    }
  }
  const report = {
    schema_version: 1,
    ok: stale_reasons.length === 0,
    current: stale_reasons.length === 0,
    certificate: resolve(options.certificate),
    checked_at: new Date().toISOString(),
    stale_reasons,
    current_boundary: boundary.sha256,
    certified_boundary: certificate.boundary?.sha256 ?? '',
  };
  await writeJSON(options.out, report);
  return report;
}

try {
  const { command, options } = argsObject(process.argv.slice(2));
  let report;
  if (command === 'fingerprint') report = await fingerprint(options);
  else if (command === 'issue') report = await issue(options);
  else if (command === 'check') report = await check(options);
  else throw new Error('usage: certificate.mjs fingerprint|issue|check [flags]');
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
  if (report.ok === false) process.exitCode = 2;
} catch (error) {
  process.stderr.write(`capturelab certificate: ${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
