// Test-only dependency injection: exercise real IPC, settings and disk spool
// without publishing UI fixture data or changing production transport policy.
/* eslint-disable @typescript-eslint/no-require-imports -- Electron CommonJS test bootstrap installs dependencies before loading main. */
const path = require('node:path');
const fs = require('node:fs');
const { app } = require('electron');
const profile = process.env.CLIPHUB_E2E_USER_DATA;
if (!profile || !path.isAbsolute(profile)) throw new Error('disposable profile required');
app.setPath('userData', profile);
delete process.env.CLIPHUB_E2E_USER_DATA;
const config = { endpoint: 'https://diagnostic-ui.invalid/v1/ingest', ingestKey: 'diagnostic-ui-fixture-public-key' };
const fetchFixture = async (_url, options) => {
  const body = JSON.parse(options.body);
  if (body.records) fs.appendFileSync(path.join(profile,'diagnostic-ui-receipts.jsonl'),body.records.map((record)=>JSON.stringify(record)+'\n').join(''));
  return {status:202,json:async()=>({accepted_ids:(body.records??[]).map((record)=>record.id)})};
};
for (const [file, name] of [['telemetry-client','TelemetryClient'],['diagnostic-log-client','DiagnosticLogClient']]) {
  const module = require(path.join(__dirname,'..','dist',file+'.js'));
  const Original = module[name];
  module[name] = class extends Original {
    constructor(options) { super({...options,release:'3.0.2',config,fetch:fetchFixture}); }
  };
}
require(path.join(__dirname,'..','dist','main.js'));
