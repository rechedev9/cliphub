import { spawnSync } from "node:child_process";
import { readFileSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { BASELINE_FILE, REPORT_ENV, REPORT_PREFIX } from "./plugin.mjs";

export function currentCommentLines(cwd) {
  const lint = JSON.parse(readFileSync(path.join(cwd, "package.json"), "utf8")).scripts.lint.split(/\s+/);
  if (lint[0] !== "oxlint") throw new Error(`${cwd}: the lint script must start with oxlint`);
  const oxlint = path.join(cwd, "node_modules", "oxlint", "bin", "oxlint");
  const run = spawnSync(process.execPath, [oxlint, ...lint.slice(1), "--format=json"], {
    cwd,
    encoding: "utf8",
    env: { ...process.env, [REPORT_ENV]: "1" },
    maxBuffer: 256 * 1024 * 1024,
  });
  const start = run.stdout.indexOf("{");
  if (start < 0) throw new Error(`oxlint gave no JSON report:\n${run.stdout}${run.stderr}`);
  const report = JSON.parse(run.stdout.slice(start));
  const counts = {};
  for (const diagnostic of report.diagnostics) {
    if (!diagnostic.message.startsWith(REPORT_PREFIX)) continue;
    const entry = diagnostic.message.slice(REPORT_PREFIX.length);
    const split = entry.lastIndexOf("=");
    counts[entry.slice(0, split)] = Number(entry.slice(split + 1));
  }
  return counts;
}

function shrunkBaseline(frozen, current) {
  const next = {};
  for (const rel of Object.keys(frozen).sort()) {
    const lines = Math.min(frozen[rel], current[rel] ?? 0);
    if (lines > 0) next[rel] = lines;
  }
  return next;
}

function main() {
  const cwd = process.cwd();
  const file = path.join(cwd, BASELINE_FILE);
  const frozen = JSON.parse(readFileSync(file, "utf8"));
  const next = shrunkBaseline(frozen, currentCommentLines(cwd));
  if (Object.keys(next).length === 0) {
    rmSync(file);
    console.log(`${BASELINE_FILE} is empty and was removed.`);
    return;
  }
  writeFileSync(file, `${JSON.stringify(next, null, 2)}\n`);
  console.log(`${BASELINE_FILE}: ${Object.keys(frozen).length} files before, ${Object.keys(next).length} now.`);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) main();
