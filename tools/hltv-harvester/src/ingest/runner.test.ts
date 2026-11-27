import { strict as assert } from "node:assert";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { describe, dryRunner, spawnRunner } from "./runner.ts";

test("spawnRunner pipes a file into the command and returns its output and exit code", async () => {
  const input = join(mkdtempSync(join(tmpdir(), "hltv-runner-")), "in.txt");
  writeFileSync(input, "hello");
  const echoed: string[] = [];
  const runner = spawnRunner((l) => echoed.push(l));
  const res = await runner.run({ cmd: process.execPath, args: ["-e", "process.stdin.pipe(process.stdout); process.stdin.on('end', () => { console.error('note'); process.exitCode = 3; })"], stdinFile: input });
  assert.deepEqual(res, { code: 3, stdout: "hello" });
  assert.deepEqual(echoed, ["note"]);
  assert.equal((await runner.run({ cmd: join(tmpdir(), "no-such-binary-here"), args: [] })).code, 127);
});

test("spawnRunner survives a command that exits before reading its input", async () => {
  const input = join(mkdtempSync(join(tmpdir(), "hltv-runner-")), "big.tar");
  writeFileSync(input, Buffer.alloc(8 * 1024 * 1024));
  const runner = spawnRunner(() => {});
  assert.equal((await runner.run({ cmd: process.execPath, args: ["-e", "process.exit(5)"], stdinFile: input })).code, 5);
  assert.equal((await runner.run({ cmd: process.execPath, args: ["-e", "process.stdin.resume()"], stdinFile: join(tmpdir(), "no-such-upload.tar") })).code, 0);
});

test("dryRunner prints the command line and runs nothing", async () => {
  const out: string[] = [];
  const runner = dryRunner((l) => out.push(l));
  let acted = false;
  assert.deepEqual(await runner.run({ cmd: "ssh", args: ["vps", "ls /a"], stdinFile: "up.tar" }), { code: 0, stdout: "" });
  runner.act("remove x", () => { acted = true; });
  assert.deepEqual(out, ['[dry-run] run ssh vps "ls /a" < up.tar', "[dry-run] remove x"]);
  assert.equal(acted, false);
  assert.equal(describe({ cmd: "go", args: ["build"], cwd: "src" }), "go build (in src)");
});
