#!/usr/bin/env node
import { runFiles } from './simulator.mjs';
import { scrubCaptureLabProcessEnvironment } from './safe-env.mjs';

scrubCaptureLabProcessEnvironment();

function usage() {
  return 'usage: node scripts/capturelab/run.mjs --script <recording.js> --scenario <scenario.json> [--out <summary.json>]';
}

function parseArgs(args) {
  const options = {};
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (!['--script', '--scenario', '--out'].includes(arg)) throw new Error(`unknown argument ${arg}`);
    const value = args[++index];
    if (!value || value.startsWith('--')) throw new Error(`${arg} requires a value`);
    if (arg === '--script') options.scriptPath = value;
    if (arg === '--scenario') options.scenarioPath = value;
    if (arg === '--out') options.outputPath = value;
  }
  if (!options.scriptPath || !options.scenarioPath) throw new Error('--script and --scenario are required');
  return options;
}

try {
  const summary = await runFiles(parseArgs(process.argv.slice(2)));
  process.stdout.write(`${JSON.stringify(summary, null, 2)}\n`);
  if (!summary.ok) process.exitCode = 2;
} catch (error) {
  process.stderr.write(`capturelab: ${error instanceof Error ? error.message : String(error)}\n${usage()}\n`);
  process.exitCode = 1;
}
