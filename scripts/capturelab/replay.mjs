#!/usr/bin/env node
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { dirname } from 'node:path';
import { runSimulation } from './simulator.mjs';

function parseArgs(args) {
  const options = {};
  const keys = new Map([
    ['--script', 'script'], ['--scenario', 'scenario'], ['--baseline', 'baseline'], ['--out', 'output'],
  ]);
  for (let index = 0; index < args.length; index++) {
    const key = keys.get(args[index]);
    if (!key) throw new Error(`unknown argument ${args[index]}`);
    const value = args[++index];
    if (!value || value.startsWith('--')) throw new Error(`${args[index - 1]} requires a value`);
    options[key] = value;
  }
  if (!options.script || !options.scenario || !options.baseline) {
    throw new Error('--script, --scenario, and --baseline are required');
  }
  return options;
}

function canonical(summary) {
  return {
    schema_version: summary.schema_version,
    outcome: summary.outcome,
    frames_executed: summary.frames_executed,
    final_tick: summary.final_tick,
    recorded_segments: summary.recorded_segments,
    disconnect_frame: summary.disconnect_frame,
    quit_frame: summary.quit_frame,
    executed_commands: summary.executed_commands,
    events: summary.events?.map(({ frame, tick, kind, value }) => ({ frame, tick, kind, value })),
  };
}

function firstDifference(expected, actual, path = '$') {
  if (Object.is(expected, actual)) return null;
  if (typeof expected !== typeof actual || expected === null || actual === null) {
    return { path, expected, actual };
  }
  if (Array.isArray(expected) || Array.isArray(actual)) {
    if (!Array.isArray(expected) || !Array.isArray(actual)) return { path, expected, actual };
    if (expected.length !== actual.length) return { path: `${path}.length`, expected: expected.length, actual: actual.length };
    for (let index = 0; index < expected.length; index++) {
      const difference = firstDifference(expected[index], actual[index], `${path}[${index}]`);
      if (difference) return difference;
    }
    return null;
  }
  if (typeof expected === 'object') {
    const keys = [...new Set([...Object.keys(expected), ...Object.keys(actual)])].sort();
    for (const key of keys) {
      const difference = firstDifference(expected[key], actual[key], `${path}.${key}`);
      if (difference) return difference;
    }
    return null;
  }
  return { path, expected, actual };
}

try {
  const options = parseArgs(process.argv.slice(2));
  const [script, scenario, baseline] = await Promise.all([
    readFile(options.script, 'utf8'),
    readFile(options.scenario, 'utf8').then(JSON.parse),
    readFile(options.baseline, 'utf8').then(JSON.parse),
  ]);
  const current = await runSimulation(script, scenario);
  const expectedCanonical = canonical(baseline);
  const actualCanonical = canonical(current);
  const difference = firstDifference(expectedCanonical, actualCanonical);
  const report = {
    schema_version: 1,
    ok: difference === null && current.ok,
    baseline: options.baseline,
    scenario: scenario.name,
    difference,
    current: actualCanonical,
  };
  if (options.output) {
    await mkdir(dirname(options.output), { recursive: true });
    await writeFile(options.output, `${JSON.stringify(report, null, 2)}\n`, 'utf8');
  }
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
  if (!report.ok) process.exitCode = 2;
} catch (error) {
  process.stderr.write(`capturelab replay: ${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
