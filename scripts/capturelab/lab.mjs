#!/usr/bin/env node
import { randomUUID } from 'node:crypto';
import { mkdir, open, readFile, rm, writeFile } from 'node:fs/promises';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import { processTreeSpawnOptions, terminateProcessTree } from './process-tree.mjs';
import { captureLabEnvironment } from './safe-env.mjs';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const validModes = new Set(['Script', 'Media', 'App', 'Studio', 'Full']);

function parseArgs(args) {
  const options = { mode: 'Full', iterations: 1, timeoutMS: 180_000 };
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    const value = args[++index];
    if (!value || value.startsWith('--')) throw new Error(`${arg} requires a value`);
    if (arg === '--mode') options.mode = value;
    else if (arg === '--iterations') options.iterations = Number(value);
    else if (arg === '--evidence-dir') options.evidenceDir = resolve(value);
    else if (arg === '--timeout-seconds') options.timeoutMS = Number(value) * 1000;
    else throw new Error(`unknown argument ${arg}`);
  }
  if (!validModes.has(options.mode)) throw new Error(`--mode must be one of ${[...validModes].join(', ')}`);
  if (!Number.isInteger(options.iterations) || options.iterations < 1 || options.iterations > 100) {
    throw new Error('--iterations must be an integer from 1 to 100');
  }
  if (!Number.isFinite(options.timeoutMS) || options.timeoutMS < 1000) throw new Error('--timeout-seconds must be >= 1');
  return options;
}

function timestamp() {
  return new Date().toISOString().replaceAll(':', '').replaceAll('.', '');
}

async function acquireRunLock() {
  const path = join(repoRoot, '.local', 'capture-lab', 'runner.lock');
  await mkdir(dirname(path), { recursive: true });
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      const handle = await open(path, 'wx', 0o600);
      await handle.writeFile(`${process.pid}\n`, 'utf8');
      return async () => {
        await handle.close();
        await rm(path, { force: true });
      };
    } catch (error) {
      if (error.code !== 'EEXIST') throw error;
      const owner = Number((await readFile(path, 'utf8').catch(() => '')).trim());
      let alive = Number.isInteger(owner) && owner > 0;
      if (alive) {
        try { process.kill(owner, 0); } catch (signalError) { if (signalError.code === 'ESRCH') alive = false; else throw signalError; }
      }
      if (alive) throw new Error(`another Capture Lab runner owns ${path} (pid ${owner})`);
      await rm(path, { force: true });
    }
  }
  throw new Error(`could not acquire Capture Lab runner lock ${path}`);
}

const maxCapturedOutputBytes = 8 * 1024 * 1024;

function boundedAppend(current, chunk) {
  const combined = Buffer.concat([current, chunk]);
  return combined.length <= maxCapturedOutputBytes ? combined : combined.subarray(combined.length - maxCapturedOutputBytes);
}

async function execute(step, executable, args, options = {}) {
  const started = Date.now();
  const logPath = join(options.evidenceDir, 'logs', `${String(options.index).padStart(2, '0')}-${step}.log`);
  await mkdir(dirname(logPath), { recursive: true });
  const command = [executable, ...args];
  const result = await new Promise((accept) => {
    const child = spawn(executable, args, {
      cwd: options.cwd ?? repoRoot,
      env: captureLabEnvironment(process.env, options.env ?? {}),
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
      ...processTreeSpawnOptions(),
    });
    let stdout = Buffer.alloc(0);
    let stderr = Buffer.alloc(0);
    child.stdout.on('data', (chunk) => { stdout = boundedAppend(stdout, chunk); });
    child.stderr.on('data', (chunk) => { stderr = boundedAppend(stderr, chunk); });
    let timedOut = false;
    let termination = Promise.resolve();
    const timer = setTimeout(() => {
      timedOut = true;
      termination = terminateProcessTree(child);
    }, options.timeoutMS);
    child.once('error', (error) => {
      clearTimeout(timer);
      accept({ code: -1, error: error.message, timedOut, stdout: '', stderr: '' });
    });
    child.once('close', async (code, signal) => {
      clearTimeout(timer);
      await termination;
      accept({
        code: code ?? -1, signal, timedOut,
        stdout: stdout.toString('utf8'),
        stderr: stderr.toString('utf8'),
      });
    });
  });
  const log = [
    `$ ${command.map((part) => JSON.stringify(part)).join(' ')}`,
    `cwd: ${options.cwd ?? repoRoot}`,
    `exit: ${result.code}${result.signal ? ` signal=${result.signal}` : ''}${result.timedOut ? ' timeout=true' : ''}`,
    '', '--- stdout ---', result.stdout, '--- stderr ---', result.stderr,
  ].join('\n');
  await writeFile(logPath, log, 'utf8');
  const record = {
    name: step, command, log: relative(options.evidenceDir, logPath).replaceAll('\\', '/'), exit_code: result.code,
    timed_out: result.timedOut, duration_ms: Date.now() - started,
    ok: result.code === 0 && !result.timedOut,
  };
  if (!record.ok) {
    const detail = result.error || result.stderr.trim() || result.stdout.trim() || `exit ${result.code}`;
    const error = new Error(`${step} failed: ${detail.slice(0, 1200)}`);
    error.stepRecord = record;
    throw error;
  }
  return { record, stdout: result.stdout, stderr: result.stderr };
}

async function executeGoTests(context, step, pkg, pattern, extraArgs = []) {
  const execution = await execute(step, 'go', [
    'test', '-json', ...extraArgs, pkg, '-run', pattern, '-count=1',
  ], {
    evidenceDir: context.evidenceDir,
    index: context.steps.length + 1,
    timeoutMS: context.options.timeoutMS,
  });
  const passedTests = execution.stdout.split(/\r?\n/).flatMap((line) => {
    try {
      const event = JSON.parse(line);
      return event.Action === 'pass' && event.Test ? [event.Test] : [];
    } catch {
      return [];
    }
  });
  if (passedTests.length === 0) {
    execution.record.ok = false;
    execution.record.executed_tests = [];
    const error = new Error(`${step} matched no Go tests for -run ${JSON.stringify(pattern)}`);
    error.stepRecord = execution.record;
    throw error;
  }
  execution.record.executed_tests = passedTests;
  return execution;
}

async function scriptPhase(context) {
  const { evidenceDir, options, steps } = context;
  const recordingDir = join(evidenceDir, 'script-recording');
  let execution = await execute('node-simulator-tests', 'node', ['--test', 'scripts/capturelab/simulator.test.mjs', 'scripts/capturelab/tools.test.mjs'], {
    evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS,
  });
  steps.push(execution.record);
  execution = await executeGoTests(context, 'go-exact-script-tests', './internal/recording', 'TestGeneratedHLAEScriptRunsInMIRVSimulator|TestFullDemoExactRuntimeInExistingMIRVSimulator');
  steps.push(execution.record);
  execution = await execute('generate-exact-script', 'go', [
    'run', './cmd/zv-recorder',
    '--killplan', 'testdata/agent-killplan.json', '--demo', 'testdata/agent-demo.fixture',
    '--out', recordingDir, '--dry-run', '--format', 'json',
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  const scenarioPath = join(evidenceDir, 'healthy-scenario.json');
  await writeFile(scenarioPath, `${JSON.stringify({
    schema_version: 1,
    name: 'agent-fixture-healthy',
    target_steamid: '76561198377256168',
    start_tick: 0,
    tick_step: 1,
    max_frames: 600,
    demo_end_tick: 640,
    expect: {
      outcome: 'verified', recorded_segments: ['seg-001'],
      must_exec: ['mirv_streams record start', 'mirv_streams record end', 'disconnect', 'quit'], soft_quit: true,
    },
  }, null, 2)}\n`, 'utf8');
  execution = await execute('exact-script-evidence', 'node', [
    'scripts/capturelab/run.mjs', '--script', join(recordingDir, 'recording.js'),
    '--scenario', scenarioPath, '--out', join(evidenceDir, 'simulator-transcript.json'),
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  execution = await execute('deterministic-trace-replay', 'node', [
    'scripts/capturelab/replay.mjs', '--script', join(recordingDir, 'recording.js'),
    '--scenario', scenarioPath, '--baseline', join(evidenceDir, 'simulator-transcript.json'),
    '--out', join(evidenceDir, 'replay-report.json'),
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  execution = await execute('capture-boundary-fingerprint', 'node', [
    'scripts/capturelab/certificate.mjs', 'fingerprint',
    '--script', join(recordingDir, 'recording.js'), '--out', join(evidenceDir, 'boundary-fingerprint.json'),
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
}

async function mediaPhase(context) {
  const { evidenceDir, options, steps } = context;
  const editorial = await executeGoTests(context, 'full-demo-editorial-media', './internal/editor', 'TestFullDemoMasterDecodedAAC|TestFullDemoSponsorAndPlaylistMediaCanary|TestFullDemoDecodedDuckingAndExplicitZero');
  steps.push(editorial.record);
  const voice = await executeGoTests(context, 'full-demo-team-voice-clock', './internal/voicecomms', 'TestDecodedTeamVoiceAfterLongSilenceAndSideChange');
  steps.push(voice.record);
  const recordingDir = join(evidenceDir, 'fake-recording');
  const renderDir = join(evidenceDir, 'render');
  const publishDir = join(evidenceDir, 'publish');
  const killPlanPath = join(evidenceDir, 'capturelab-killplan.json');
  const demoPath = join(evidenceDir, 'capturelab-demo.fixture');
  const killPlan = JSON.parse(await readFile(resolve(repoRoot, 'testdata', 'agent-killplan.json'), 'utf8'));
  // Fake capture creates five-second clips. Keep its source clock and the
  // editorial tick windows identical so the oracle checks actual hard cuts.
  killPlan.segments[0].tick_end = 384;
  killPlan.segments[0].kills[0].tick = 128;
  // Preserve the legacy one-second protected lead and leave EOF headroom.
  killPlan.demo.duration_ticks = 1024;
  const secondSegment = structuredClone(killPlan.segments[0]);
  secondSegment.id = 'seg-002';
  secondSegment.round = 2;
  secondSegment.tick_start = 448;
  secondSegment.tick_end = 768;
  secondSegment.kills[0].tick = 512;
  secondSegment.kills[0].weapon = 'm4a1';
  secondSegment.kills[0].victim.steamid64 = '76561198000000002';
  secondSegment.kills[0].victim.name_in_demo = 'second-opponent';
  killPlan.segments.push(secondSegment);
  killPlan.stats.kills_after_filters = 2;
  killPlan.stats.segments_created = 2;
  killPlan.stats.duration_seconds_total = 10;
  await Promise.all([
    writeFile(killPlanPath, `${JSON.stringify(killPlan, null, 2)}\n`, 'utf8'),
    writeFile(demoPath, await readFile(resolve(repoRoot, 'testdata', 'agent-demo.fixture'))),
  ]);
  let execution = await executeGoTests(context, 'go-instrumentation-tests', './cmd/zv-recorder', 'TestCaptureLab');
  steps.push(execution.record);
  execution = await execute('instrumented-fake-capture', 'go', [
    'run', './cmd/zv-recorder',
    '--killplan', killPlanPath, '--demo', demoPath,
    '--out', recordingDir, '--fake', '--fps', '60', '--format', 'json',
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  execution = await execute('source-media-oracle', 'node', [
    'scripts/capturelab/media-oracle.mjs',
    '--recording-result', join(recordingDir, 'recording-result.json'),
    '--instrumentation', join(recordingDir, 'capturelab-instrumentation.json'),
    '--out', join(evidenceDir, 'source-media-report.json'),
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  execution = await execute('real-synthetic-render', 'go', [
    'run', './cmd/zv-editor',
    '--recording-result', join(recordingDir, 'recording-result.json'),
    '--killplan', killPlanPath,
    '--out', renderDir, '--publish-dir', publishDir,
    '--compile-segments', '--preset', 'viral-60-clean', '--output-format', 'short-9x16',
    '--kill-effect', 'clean', '--transition', 'cut',
    '--hook=false', '--kill-counter=false', '--intro=false', '--outro=false',
    '--killfeed-overlay=false', '--covers=false', '--cover-sheets=false',
    '--tail-trim', '0', '--video-preset', 'veryfast', '--format', 'json',
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  const rendered = JSON.parse(await readFile(join(renderDir, 'shorts-result.json'), 'utf8'));
  const seedPath = join(evidenceDir, 'capturelab-studio-seed.json');
  await writeFile(seedPath, `${JSON.stringify({
    schema_version: 1,
    job_id: '11111111-1111-4111-8111-111111111111',
    demo_file_name: 'capturelab.dem',
    variant: 'viral-60-clean',
    killplan_path: killPlanPath,
    recording_result_path: join(recordingDir, 'recording-result.json'),
    shorts_result_path: join(renderDir, 'shorts-result.json'),
    pack_manifest_path: join(publishDir, 'pack-manifest.json'),
    gallery_path: join(publishDir, 'index.html'),
    publish_summary_path: join(publishDir, 'publish-summary.md'),
    expected_video_path: rendered.shorts[0].publish_path,
  }, null, 2)}\n`, 'utf8');
  execution = await execute('render-media-oracle', 'node', [
    'scripts/capturelab/media-oracle.mjs',
    '--recording-result', join(recordingDir, 'recording-result.json'),
    '--instrumentation', join(recordingDir, 'capturelab-instrumentation.json'),
    '--shorts-result', join(renderDir, 'shorts-result.json'),
    '--out', join(evidenceDir, 'render-media-report.json'),
  ], { evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS });
  steps.push(execution.record);
  execution = await executeGoTests(context, 'fake-capture-production-gate', './internal/recording', 'TestValidateRunResultRejectsNonReusableCaptures');
  steps.push(execution.record);
}

async function appPhase(context) {
  const { evidenceDir, options, steps } = context;
  for (const [name, pkg, pattern, extraArgs = []] of [
    ['cli-demo-journey', './cmd/zv', 'TestDemoJourneyChainsStagesMediaFree'],
    ['real-http-render-journey', './cmd/zv-orchestrator', 'TestEditorRenderE2E'],
    ['inline-queue-http-journey', './cmd/zv-orchestrator', 'TestInlineQueueShutdownCompensatesAccepted.*ThroughHTTP'],
    ['full-demo-plan-admission', './internal/httpapi', 'TestFullDemo'],
    ['full-demo-durable-reuse', './internal/workers', 'TestFullDemo|TestRenderVariantOutputsReady'],
    ['capturelab-seed-boundary', './cmd/zv-orchestrator', 'TestSeedCaptureLab|TestCaptureLabBuild', ['-tags', 'capturelab']],
  ]) {
    const execution = await executeGoTests(context, name, pkg, pattern, extraArgs);
    steps.push(execution.record);
  }
}

async function studioPhase(context) {
  const { evidenceDir, options, steps } = context;
  let execution = await execute('live-studio-orchestrator-journey', 'node', [
    'scripts/capturelab/live-studio.mjs',
    '--seed', join(evidenceDir, 'capturelab-studio-seed.json'),
    '--evidence-dir', join(evidenceDir, 'live-studio'),
    '--timeout-seconds', String(Math.ceil(options.timeoutMS / 1000)),
  ], {
    evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS,
  });
  steps.push(execution.record);
  // Use the Windows shim through cmd with fixed, repository-owned arguments.
  const playwrightArgs = ['--dir', 'web', 'exec', 'playwright', 'test', 'e2e/full-demo.spec.ts', 'e2e/library.spec.ts', '--reporter=line'];
  execution = await execute('studio-playwright-journeys', process.platform === 'win32' ? 'cmd.exe' : 'pnpm',
    process.platform === 'win32' ? ['/d', '/s', '/c', `pnpm ${playwrightArgs.join(' ')}`] : playwrightArgs, {
    evidenceDir, index: steps.length + 1, timeoutMS: options.timeoutMS,
    env: {
      PLAYWRIGHT_OUTPUT_DIR: join(evidenceDir, 'playwright-results'),
      PLAYWRIGHT_HTML_OUTPUT_DIR: join(evidenceDir, 'playwright-report'),
    },
  });
  steps.push(execution.record);
}

async function writeSummary(context, error) {
  const ended = new Date();
  const summary = {
    schema_version: 1,
    run_id: context.runID,
    mode: context.options.mode,
    iteration: context.iteration,
    started_at: context.started.toISOString(),
    ended_at: ended.toISOString(),
    duration_ms: ended.getTime() - context.started.getTime(),
    ok: !error && context.steps.every((step) => step.ok),
    highest_level: context.highestLevel,
    hlae_cs2_recertified: false,
    compatibility_statement: 'HLAE/CS2 was not launched; external compatibility is not recertified.',
    application_scope: ['App', 'Studio', 'Full'].includes(context.options.mode)
      ? 'Application stages are composed evidence: HTTP queue/render tests plus a build-tagged, in-memory ready-result seed served through the real orchestrator, same-origin proxy, and Studio. This is not one continuous production worker lifecycle.'
      : 'Application/service boundaries were not requested in this mode.',
    limitations: [
      'Synthetic and fake captures remain ineligible for production reuse or upload-ready status.',
      'The Studio boundary starts from an already validated synthetic render; capture and render worker lifecycle checks run as separate application tests.',
      'A real HLAE/CS2 canary is required for L5 compatibility evidence.',
    ],
    error: error?.message ?? '',
    steps: context.steps,
  };
  await writeFile(join(context.evidenceDir, 'summary.json'), `${JSON.stringify(summary, null, 2)}\n`, 'utf8');
  const markdown = [
    '# Capture Lab summary', '',
    `- Result: **${summary.ok ? 'PASS' : 'FAIL'}**`,
    `- Mode: ${summary.mode}`,
    `- Highest completed confidence: ${summary.highest_level}`,
    `- Duration: ${summary.duration_ms} ms`,
    `- HLAE/CS2 recertified: no`,
    `- Statement: ${summary.compatibility_statement}`,
    `- Application scope: ${summary.application_scope}`, '',
    '## Limitations', '',
    ...summary.limitations.map((limitation) => `- ${limitation}`), '',
    '## Steps', '',
    ...summary.steps.map((step) => `- ${step.ok ? 'PASS' : 'FAIL'} — ${step.name} (${step.duration_ms} ms) — \`${step.log}\``),
    ...(summary.error ? ['', '## Failure', '', summary.error] : []), '',
  ].join('\n');
  await writeFile(join(context.evidenceDir, 'SUMMARY.md'), markdown, 'utf8');
  return summary;
}

async function runIteration(options, iteration, baseEvidenceDir) {
  const evidenceDir = options.iterations === 1 ? baseEvidenceDir : join(baseEvidenceDir, `iteration-${String(iteration).padStart(3, '0')}`);
  await mkdir(evidenceDir, { recursive: true });
  const context = {
    options, iteration, evidenceDir, runID: randomUUID(), started: new Date(), steps: [], highestLevel: 'L0',
  };
  let failure;
  try {
    if (['Script', 'Full'].includes(options.mode)) {
      await scriptPhase(context);
      context.highestLevel = 'L2';
    }
    if (['Media', 'Studio', 'Full'].includes(options.mode)) {
      await mediaPhase(context);
      context.highestLevel = 'L3';
    }
    if (['App', 'Studio', 'Full'].includes(options.mode)) {
      await appPhase(context);
      context.highestLevel = 'L4';
    }
    if (['Studio', 'Full'].includes(options.mode)) {
      await studioPhase(context);
      context.highestLevel = 'L4';
    }
  } catch (error) {
    failure = error;
    if (error.stepRecord) context.steps.push(error.stepRecord);
  }
  const summary = await writeSummary(context, failure);
  process.stdout.write(`[Capture Lab] iteration ${iteration}: ${summary.ok ? 'PASS' : 'FAIL'} — ${evidenceDir}\n`);
  if (!summary.ok) throw failure ?? new Error('Capture Lab failed');
  return summary;
}

let releaseRunLock;
try {
  const options = parseArgs(process.argv.slice(2));
  releaseRunLock = await acquireRunLock();
  const baseEvidenceDir = options.evidenceDir ?? join(repoRoot, '.local', 'capture-lab', `${timestamp()}-${options.mode.toLowerCase()}`);
  for (let iteration = 1; iteration <= options.iterations; iteration++) {
    await runIteration(options, iteration, baseEvidenceDir);
  }
} catch (error) {
  process.stderr.write(`Capture Lab: ${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
} finally {
  await releaseRunLock?.();
}
