#!/usr/bin/env node
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const SUPPORT = /^CH(?:-[A-F0-9]{4}){5}$/;

/** Query remote evidence, including gaps, without access to the user's machine. */
export async function collectDiagnostics(filters, request, maxRecords = 100_000) {
  if (!filters.job_id && !filters.support_code && !filters.session_id) throw new Error('Specify a job, support code or session');
  if (filters.job_id && !UUID.test(filters.job_id) || filters.session_id && !UUID.test(filters.session_id) ||
    filters.support_code && !SUPPORT.test(filters.support_code)) throw new Error('Invalid diagnostic identity');
  let complete = true;
  const all = new Map();
  const supports = new Set(filters.support_code ? [filters.support_code] : []);
  const sessions = new Set(filters.session_id ? [filters.session_id] : []);
  async function logs(query) {
    let after = 0;
    for (;;) {
      const params = new URLSearchParams({ ...query, since: filters.since ?? new Date(Date.now() - 30 * 86_400_000).toISOString(), after: String(after), limit: '500' });
      const page = await request(`/v1/logs?${params}`);
      if (!Array.isArray(page.records) || !Number.isSafeInteger(page.next_cursor) || typeof page.has_more !== 'boolean') throw new Error('Invalid log query response');
      for (const record of page.records) {
        if (all.size >= maxRecords && !all.has(record.id)) { complete = false; return; }
        // Other jobs may run in the same session; retain only unassigned
        // context and the requested job, and label context separately below.
        if (filters.job_id && record.job_id && record.job_id !== filters.job_id) continue;
        all.set(record.id, record);
      }
      if (!page.has_more) return;
      if (page.next_cursor <= after) throw new Error('Log pagination did not advance');
      after = page.next_cursor;
    }
  }
  const primary = { ...filters };
  delete primary.since;
  await logs(primary);
  const legacyParams = new URLSearchParams({ limit: '200' });
  if (filters.job_id) legacyParams.set('job_id', filters.job_id);
  if (filters.support_code) legacyParams.set('support_code', filters.support_code);
  if (filters.since) legacyParams.set('since', filters.since);
  const legacy = filters.job_id || filters.support_code ? await request(`/v1/incidents?${legacyParams}`) : { events: [] };
  if (!Array.isArray(legacy.events)) throw new Error('Invalid incident query response');
  for (const record of [...all.values(), ...legacy.events]) {
    if (SUPPORT.test(record.support_code)) supports.add(record.support_code);
    if (UUID.test(record.session_id)) sessions.add(record.session_id);
  }
  if (filters.job_id) {
    for (const session of sessions) await logs({ session_id: session });
    // Gaps may be reported only on the next boot after an offline crash. They
    // are installation evidence, not proof that this specific job lost data.
    for (const support of supports) {
      for (const event of ['delivery.gap', 'delivery.rejected', 'delivery.health']) await logs({ support_code: support, event });
    }
  }
  const records = [...all.values()].sort((a, b) => a.cursor - b.cursor);
  return summarizeDiagnostics({ filters, records, legacyEvents: legacy.events,
    retrievalComplete: complete, legacyLimitReached: legacy.events.length === 200 });
}

export function summarizeDiagnostics({ filters, records, legacyEvents = [], retrievalComplete = true, legacyLimitReached = false }) {
  const attempts = new Map();
  for (const record of records) {
    if (!record.attempt_id || !/^(attempt\.|process\.)/.test(record.event)) continue;
    const key = `${record.support_code}:${record.session_id}:${record.attempt_id}`;
    let attempt = attempts.get(key);
    if (!attempt) {
      attempt = { job_id: record.job_id ?? null, attempt_id: record.attempt_id, session_id: record.session_id,
        operation: record.operation ?? 'unknown', attempt: record.attempt ?? null,
        start_record_present: false, state: 'no_terminal_record', first_sequence: record.sequence, last_sequence: record.sequence };
      attempts.set(key, attempt);
    }
    attempt.last_sequence = Math.max(attempt.last_sequence, record.sequence);
    if (record.event === 'attempt.started') attempt.start_record_present = true;
    if (record.event === 'attempt.finished') {
      attempt.state = record.outcome ?? 'unknown';
      attempt.duration_ms = record.duration_ms ?? null;
      attempt.terminal_record_id = record.id;
    }
  }
  const primaryRecords = filters.job_id ? records.filter((record) => record.job_id === filters.job_id) : records;
  const failure = primaryRecords.find((record) => record.level === 'error' && !record.event.startsWith('delivery.')) ?? null;
  const relevant = failure?.attempt_id ? records.filter((record) => record.attempt_id === failure.attempt_id) : records;
  const index = failure ? relevant.findIndex((record) => record.id === failure.id) : -1;
  const gaps = records.filter((record) => record.event === 'delivery.gap' || record.event === 'delivery.rejected');
  return {
    generated_at: new Date().toISOString(), filters,
    evidence: {
      retrieval_complete: retrievalComplete, legacy_limit_reached: legacyLimitReached,
      log_records: records.length, legacy_events: legacyEvents.length,
      first_failure: failure, failure_context: index >= 0 ? relevant.slice(Math.max(0, index - 40), index + 21) : [],
      installation_delivery_gaps: gaps,
      latest_delivery_health: records.filter((record) => record.event === 'delivery.health').at(-1) ?? null,
      note: 'No terminal record means unverified completion. Installation gaps may affect other jobs. No logs from an older client is not evidence of success.',
    },
    attempts: [...attempts.values()], records, legacy_events: legacyEvents,
  };
}

function configuration() {
  const file = process.env.CLIPHUB_TELEMETRY_AGENT_ENV ?? path.join(os.homedir(), '.config', 'cliphub', 'telemetry-agent.env');
  const values = {};
  if (fs.existsSync(file)) {
    for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
      const match = /^(?:export\s+)?(CLIPHUB_TELEMETRY_ADMIN_URL|CLIPHUB_TELEMETRY_ADMIN_TOKEN)=(.*)$/.exec(line.trim());
      if (match) values[match[1]] = match[2].replace(/^(['"])(.*)\1$/, '$2');
    }
  }
  const base = process.env.CLIPHUB_TELEMETRY_ADMIN_URL ?? values.CLIPHUB_TELEMETRY_ADMIN_URL;
  const token = process.env.CLIPHUB_TELEMETRY_ADMIN_TOKEN ?? values.CLIPHUB_TELEMETRY_ADMIN_TOKEN;
  if (!base || !token || token.length < 32) throw new Error('Configure the telemetry admin URL and token in the environment or telemetry-agent.env');
  const url = new URL(base);
  if (url.username || url.password || url.protocol !== 'https:' && !(url.protocol === 'http:' && ['127.0.0.1', '[::1]'].includes(url.hostname))) throw new Error('Admin URL must use HTTPS or numeric loopback');
  return { url, token };
}

async function main() {
  const args = process.argv.slice(2);
  const options = {};
  for (let index = 0; index < args.length; index += 2) {
    if (!['--job','--support','--session','--since','--out'].includes(args[index]) || !args[index + 1]) throw new Error('Usage: node scripts/telemetry-debug.mjs --job UUID | --support CH-... | --session UUID [--since ISO8601] [--out report.json]');
    options[args[index]] = args[index + 1];
  }
  const { url, token } = configuration();
  const filters = {};
  for (const [flag, key] of [['--job','job_id'],['--support','support_code'],['--session','session_id'],['--since','since']]) if (options[flag]) filters[key] = options[flag];
  const report = await collectDiagnostics(filters, async (requestPath) => {
    const response = await fetch(new URL(requestPath, url), { headers: { Authorization: `Bearer ${token}` }, redirect: 'error', signal: AbortSignal.timeout(15_000) });
    if (!response.ok) throw new Error(`Collector query failed: HTTP ${response.status}`);
    return response.json();
  });
  const output = `${JSON.stringify(report, null, 2)}\n`;
  if (options['--out']) {
    fs.writeFileSync(options['--out'], output, { flag: 'wx', mode: 0o600 });
    process.stdout.write(`Saved ${report.records.length} logs, ${report.legacy_events.length} legacy events, ${report.attempts.length} attempts. Pagination complete: ${report.evidence.retrieval_complete}.\n`);
  } else process.stdout.write(output);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => { process.stderr.write(`${error.message}\n`); process.exitCode = 1; });
}
