import assert from "node:assert/strict";
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";
import { ConfigError, loadConfig } from "./config.ts";

const SRC = import.meta.dirname;

function withConfig(body: unknown, run: (file: string, dir: string) => void): void {
  const dir = mkdtempSync(path.join(tmpdir(), "cliphub-content-config-"));
  try {
    const file = path.join(dir, "config.local.json");
    writeFileSync(file, JSON.stringify(body));
    run(file, dir);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test("env wins over the config file, which expands %VARS% and resolves relative paths", () => {
  withConfig({ statePath: "%DATA%\\state.db", legacyMapsDir: "maps", studioMcp: { command: "zv-mcp" } }, (file, dir) => {
    const fromFile = loadConfig({ env: { CLIPHUB_CONTENT_CONFIG: file, DATA: dir } });
    assert.equal(fromFile.configFile, file);
    assert.equal(fromFile.statePath, path.resolve(dir, `${dir}\\state.db`));
    assert.equal(fromFile.legacyMapsDir, path.join(dir, "maps"));
    assert.deepEqual(fromFile.studioMcp, { command: "zv-mcp", args: [] });
    const fromEnv = loadConfig({ env: { CLIPHUB_CONTENT_CONFIG: file, DATA: dir, CLIPHUB_CONTENT_STATE_PATH: path.join(dir, "other.db") } });
    assert.equal(fromEnv.statePath, path.join(dir, "other.db"));
    assert.throws(() => loadConfig({ env: { CLIPHUB_CONTENT_CONFIG: file } }), ConfigError);
  });
});

test("without a config file the state lives under LOCALAPPDATA and nothing else is set", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cliphub-content-empty-"));
  try {
    const config = loadConfig({ env: { LOCALAPPDATA: dir }, packageDir: dir });
    assert.deepEqual(config, {
      configFile: null,
      statePath: path.join(dir, "cliphub-content", "state.db"),
      legacyMapsDir: null,
      legacyMetricsDir: null,
      studioMcp: null,
      renderBin: null,
    });
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("a config file with an unknown key is refused", () => {
  withConfig({ statePath: "x.db", tiktokDir: "y" }, (file) => {
    assert.throws(() => loadConfig({ env: { CLIPHUB_CONTENT_CONFIG: file } }), ConfigError);
  });
});

test("no source file names a user's home folder", () => {
  const drive = ["C:", "Users"];
  const needles = [drive.join("\\"), drive.join("\\\\"), drive.join("/")].map((n) => n.toLowerCase());
  const files = readdirSync(SRC, { recursive: true, encoding: "utf8" }).filter((name) => name.endsWith(".ts"));
  assert.ok(files.length > 5);
  const offenders = files.filter((name) => {
    const source = readFileSync(path.join(SRC, name), "utf8").toLowerCase();
    return needles.some((needle) => source.includes(needle));
  });
  assert.deepEqual(offenders, []);
});
