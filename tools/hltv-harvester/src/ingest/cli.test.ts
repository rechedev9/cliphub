import { strict as assert } from "node:assert";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fakeCommunity } from "../testing/fake-community.ts";
import { main } from "./cli.ts";
import { describe, type Command, type Result, type Runner } from "./runner.ts";

type Script = (c: Command, line: string) => Result | undefined;

function fakeRunner(script: Script = () => undefined): Runner & { ran: string[]; uploads: string[] } {
  const ran: string[] = [];
  const uploads: string[] = [];
  return {
    dryRun: false,
    ran,
    uploads,
    run: async (c) => {
      const line = describe(c);
      ran.push(line);
      if (c.cmd === "tar" && c.args[0] === "cf" && c.cwd !== undefined) uploads.push(readFileSync(join(c.cwd, c.args[3] ?? ""), "utf8").trim());
      return script(c, line) ?? { code: 0, stdout: "" };
    },
    act: (_d, fn) => fn(),
  };
}

function home(): string {
  return mkdtempSync(join(tmpdir(), "hltv-ingest-"));
}

function env(dir: string, url: string): NodeJS.ProcessEnv {
  return { CLIPHUB_HLTV_HOME: dir, CLIPHUB_COMMUNITY_URL: url, CLIPHUB_ELECTRON: "fake-electron", CLIPHUB_VPS_SSH: "vps" };
}

function touch(path: string): void {
  mkdirSync(join(path, ".."), { recursive: true });
  writeFileSync(path, "x");
}

const ID = 2400507;

function harvesterAndPrepare(dir: string): Script {
  return (c) => {
    if (c.cmd === "fake-electron") touch(join(dir, "downloads", `${ID}.rar`));
    if (c.args[0] === "prepare-hltv") {
      touch(join(dir, "analyses", `${ID}__a-vs-b-m1-nuke.zvpro.zst`));
      touch(join(dir, "analyses", `${ID}__a-vs-b-m2-inferno.zvpro.zst`));
    }
    if (c.cmd === "ssh" && c.args[1]?.startsWith("ls ")) return { code: 0, stdout: `/var/lib/cliphub-community/hltv:\n${ID}__a-vs-b-m1-nuke.zvpro.zst\n` };
    return undefined;
  };
}

test("match downloads a missing match, keeps its demos and uploads only the maps the VPS lacks", async () => {
  const dir = home();
  const api = await fakeCommunity(new Set());
  const runner = fakeRunner(harvesterAndPrepare(dir));
  assert.equal(await main(["match", String(ID)], { env: env(dir, api.url), print: () => {}, runner }), 0);
  const electron = runner.ran.find((l) => l.startsWith("fake-electron"));
  assert.match(electron ?? "", new RegExp(`--match ${ID} --out .+ --community-url ${api.url.replaceAll(".", "\\.")}`));
  assert.ok(runner.ran.some((l) => l.includes("prepare-hltv") && l.includes("-keep-demos")));
  assert.deepEqual(runner.uploads, [`${ID}__a-vs-b-m2-inferno.zvpro.zst`]);
  assert.match(runner.ran.at(-1) ?? "", /^ssh vps "sudo tar xf - -C \/var\/lib\/cliphub-community\/hltv .*cliphub-community-index.service" < /);
  assert.deepEqual(api.calls, [`/v1/pro-match-ids?ids=${ID}`]);
  assert.equal(existsSync(join(dir, "ingest.lock")), false);
});

test("match with a kept demo and a match the VPS registered downloads and uploads nothing", async () => {
  const dir = home();
  touch(join(dir, "demos", `${ID}__a-vs-b-m1-nuke.dem`));
  const api = await fakeCommunity(new Set([ID]));
  const runner = fakeRunner(harvesterAndPrepare(dir));
  assert.equal(await main(["match", String(ID)], { env: env(dir, api.url), print: () => {}, runner }), 0);
  assert.equal(runner.ran.some((l) => l.startsWith("fake-electron") || l.startsWith("ssh")), false);
  assert.ok(runner.ran.some((l) => l.includes("prepare-hltv")));
});

test("match goes on with the VPS listing when the community API is down", async () => {
  const dir = home();
  const api = await fakeCommunity(new Set([ID]), 503);
  const runner = fakeRunner(harvesterAndPrepare(dir));
  const out: string[] = [];
  assert.equal(await main(["match", String(ID)], { env: env(dir, api.url), print: (l) => out.push(l), runner }), 0);
  assert.ok(out.some((l) => l.includes("class=registry_unavailable")));
  assert.deepEqual(runner.uploads, [`${ID}__a-vs-b-m2-inferno.zvpro.zst`]);
});

test("match fails when the harvester saves no demo, before anything reaches the VPS", async () => {
  const dir = home();
  const api = await fakeCommunity(new Set());
  const runner = fakeRunner();
  assert.equal(await main(["match", String(ID)], { env: env(dir, api.url), print: () => {}, runner }), 1);
  assert.equal(runner.ran.some((l) => l.startsWith("ssh") || l.includes("prepare-hltv")), false);
});

test("match --dry-run prints every step and runs, writes and uploads nothing", async () => {
  const dir = home();
  const api = await fakeCommunity(new Set());
  const out: string[] = [];
  assert.equal(await main(["match", String(ID), "--dry-run"], { env: env(dir, api.url), print: (l) => out.push(l) }), 0);
  const text = out.join("\n");
  for (const step of ["take the lock", "run pnpm run build", `run fake-electron`, `--match ${ID}`, "prepare-hltv", "-keep-demos", `ssh vps "ls `, `${ID}__*.zvpro.zst`, "tar cf ../upload.tar", "sudo systemctl start cliphub-community-index.service", "remove "]) {
    assert.ok(text.includes(step), `dry run lacks ${step}:\n${text}`);
  }
  assert.deepEqual(readdirSync(dir), []);
});

test("daily keeps the order of daily.sh, survives a failed harvest and uploads new analyses and clips", async () => {
  const dir = home();
  touch(join(dir, "analyses", "1__old.zvpro.zst"));
  touch(join(dir, "analyses", "2__new.zvpro.zst"));
  touch(join(dir, "clips", "de_nuke", "a.mp4"));
  touch(join(dir, "clips", "de_nuke", "b.mp4"));
  touch(join(dir, "clips", "de_nuke", "c.tmp.mp4"));
  const runner = fakeRunner((c) => {
    if (c.cmd === "fake-electron") return { code: 1, stdout: "" };
    if (c.cmd === "go" && c.args[0] === "build") touch(c.args[2] ?? "");
    if (c.cmd === "ssh" && c.args[1]?.startsWith("ls ")) return { code: 0, stdout: "1__old.zvpro.zst\n" };
    if (c.cmd === "ssh" && c.args[1]?.includes("find /var/lib/cliphub-community/clips -name")) return { code: 0, stdout: "de_nuke/a.mp4 1\n" };
    return undefined;
  });
  const out: string[] = [];
  assert.equal(await main(["daily"], { env: env(dir, "off"), print: (l) => out.push(l), runner }), 0);
  const order = ["pnpm run build", "fake-electron", "git -C", "go build", "prepare-hltv", "ssh vps \"ls ", "tar cf ../upload.tar", "sudo tar xf - -C", "sudo find", "tar cf ../clips-batch.tar", "--no-same-owner", "chown -R", "build-lineups"];
  let at = -1;
  for (const step of order) {
    const next = runner.ran.findIndex((l, i) => i > at && l.includes(step));
    assert.ok(next > at, `${step} out of order:\n${runner.ran.join("\n")}`);
    at = next;
  }
  assert.ok(runner.ran.find((l) => l.startsWith("fake-electron"))?.includes("--ranked-only --out"));
  assert.ok(runner.ran.find((l) => l.startsWith("fake-electron"))?.includes("--community-url off"));
  assert.deepEqual(runner.uploads, ["2__new.zvpro.zst", "de_nuke/b.mp4"]);
  assert.ok(out.some((l) => l.includes("harvester exited with 1")));
  assert.ok(existsSync(join(dir, "cliphub-community.exe")) || existsSync(join(dir, "cliphub-community")));
  assert.equal(existsSync(join(dir, ".build-src")), false);
  assert.ok(readFileSync(join(dir, "ingest.log"), "utf8").includes("== done"));
});

test("daily stops before the VPS when prepare-hltv fails, and a failed parser build keeps the old binary", async () => {
  const dir = home();
  const runner = fakeRunner((c) => (c.args[0] === "prepare-hltv" || (c.cmd === "go" && c.args[0] === "build") ? { code: 1, stdout: "" } : undefined));
  const out: string[] = [];
  assert.equal(await main(["daily"], { env: env(dir, "off"), print: (l) => out.push(l), runner }), 1);
  assert.equal(runner.ran.some((l) => l.startsWith("ssh")), false);
  assert.ok(out.some((l) => l.includes("keeping the current cliphub-community")));
});

test("a second run waits for none: it stops while a live run holds the lock", async () => {
  const dir = home();
  writeFileSync(join(dir, "ingest.lock"), String(process.pid));
  const runner = fakeRunner();
  assert.equal(await main(["daily"], { env: env(dir, "off"), print: () => {}, runner }), 1);
  assert.deepEqual(runner.ran, []);
});

test("bad arguments print the usage", async () => {
  const out: string[] = [];
  for (const argv of [[], ["match"], ["match", "abc"], ["daily", "1"], ["sweep"]]) {
    assert.equal(await main(argv, { env: {}, print: (l) => out.push(l), runner: fakeRunner() }), 2);
  }
  assert.ok(out.every((l) => l.startsWith("usage:")));
});
