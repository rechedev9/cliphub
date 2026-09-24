import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import {
  classifyExit,
  clearSessionMarker,
  countMinidumps,
  EXIT_DLL_INIT_FAILED,
  EXIT_SESSION_TERMINATED,
  markSessionShuttingDown,
  recordPreviousSession,
  takeCrashOutput,
} from './crash-monitor.ts';
import type { DiagnosticLogInput } from './diagnostic-log.ts';

function tempDir(t: test.TestContext): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-crash-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

function start(dir: string, electronVersion = '43.0.0'): { records: DiagnosticLogInput[]; logs: string[]; result: ReturnType<typeof recordPreviousSession> } {
  const records: DiagnosticLogInput[] = [];
  const logs: string[] = [];
  const result = recordPreviousSession({
    markerPath: path.join(dir, 'session-marker.json'),
    crashDumpsDir: path.join(dir, 'Crashpad'),
    electronVersion,
    record: (input) => records.push(input),
    log: (text) => logs.push(text),
  });
  return { records, logs, result };
}

function addMinidump(dir: string, name: string): void {
  const reports = path.join(dir, 'Crashpad', 'reports');
  fs.mkdirSync(reports, { recursive: true });
  fs.writeFileSync(path.join(reports, name), 'MDMP');
}

test('classifies Windows session termination as a shutdown kill', () => {
  assert.equal(EXIT_SESSION_TERMINATED, 0x40010004);
  assert.equal(EXIT_DLL_INIT_FAILED, 0xC0000142);
  assert.equal(classifyExit(1073807364, false), 'shutdown_kill');
  assert.equal(classifyExit(1073807364, true), 'shutdown_kill');
  assert.equal(classifyExit(3221225794, true), 'shutdown_kill');
  assert.equal(classifyExit(3221225794, false), 'crash');
  assert.equal(classifyExit(1, true), 'crash');
  assert.equal(classifyExit(2, false), 'crash');
  assert.equal(classifyExit(null, false), 'crash');
});

test('a first start and a clean quit report nothing', (t) => {
  const dir = tempDir(t);
  const first = start(dir);
  assert.deepEqual(first.records, []);
  assert.equal(first.result.crashed, false);
  assert.ok(fs.existsSync(path.join(dir, 'session-marker.json')));

  clearSessionMarker(path.join(dir, 'session-marker.json'));
  addMinidump(dir, 'renderer-reported-live.dmp');
  const second = start(dir);
  assert.deepEqual(second.records, []);
});

test('a leftover marker reports the previous session with its minidump', (t) => {
  const dir = tempDir(t);
  addMinidump(dir, 'older.dmp');
  start(dir, '42.1.0');
  addMinidump(dir, 'main-process.dmp');

  const next = start(dir);
  assert.deepEqual(next.result, { crashed: true, minidump: true });
  assert.deepEqual(next.records, [{
    event: 'session.crashed_previous',
    level: 'error',
    message: 'process_type=browser minidump=yes electron=v42.1.0',
  }]);

  // The new marker counts the dump, so a later crash without one says so.
  const after = start(dir);
  assert.equal(after.records[0]?.message, 'process_type=browser minidump=no electron=v43.0.0');
});

test('a session ended by Windows is not a crash', (t) => {
  const dir = tempDir(t);
  start(dir);
  markSessionShuttingDown(path.join(dir, 'session-marker.json'));
  const next = start(dir);
  assert.deepEqual(next.records, []);
  assert.equal(next.result.crashed, false);
  assert.match(next.logs.join(''), /ended with the Windows session/);
});

test('an unreadable marker still counts as an unclean exit', (t) => {
  const dir = tempDir(t);
  fs.writeFileSync(path.join(dir, 'session-marker.json'), '{"version":1,');
  const next = start(dir);
  assert.equal(next.records[0]?.message, 'process_type=browser minidump=no electron=unknown');
});

test('counts dumps at the Crashpad root and one level down only', (t) => {
  const dir = tempDir(t);
  const crashpad = path.join(dir, 'Crashpad');
  fs.mkdirSync(path.join(crashpad, 'reports', 'deeper'), { recursive: true });
  fs.writeFileSync(path.join(crashpad, 'a.dmp'), '');
  fs.writeFileSync(path.join(crashpad, 'reports', 'b.DMP'), '');
  fs.writeFileSync(path.join(crashpad, 'reports', 'settings.dat'), '');
  fs.writeFileSync(path.join(crashpad, 'reports', 'deeper', 'c.dmp'), '');
  assert.equal(countMinidumps(crashpad), 2);
  assert.equal(countMinidumps(path.join(dir, 'missing')), 0);
});

test('takes the Go crash output once and keeps its head and tail', (t) => {
  const dir = tempDir(t);
  const file = path.join(dir, 'orchestrator-crash.log');
  assert.equal(takeCrashOutput(file), null);

  fs.writeFileSync(file, '');
  assert.equal(takeCrashOutput(file), null);
  assert.equal(fs.existsSync(file), false);

  fs.writeFileSync(file, 'panic: boom\r\n\r\ngoroutine 1 [running]:\r\nmain.main()\r\n');
  assert.equal(takeCrashOutput(file), 'panic: boom\n\ngoroutine 1 [running]:\nmain.main()');
  assert.equal(fs.existsSync(file), false);

  const frames = Array.from({ length: 400 }, (_, index) => `frame ${index}`);
  fs.writeFileSync(file, ['fatal error: concurrent map writes', ...frames].join('\n'));
  const excerpt = takeCrashOutput(file) ?? '';
  const lines = excerpt.split('\n');
  assert.equal(lines[0], 'fatal error: concurrent map writes');
  assert.equal(lines[30], '[221 lines omitted]');
  assert.equal(lines.at(-1), 'frame 399');
  assert.equal(lines.length, 181);
});
