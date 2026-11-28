import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";
import { at, openHarness } from "../testing/harness.ts";

function writeFakeRenderCli(dir: string): string {
  const script = path.join(dir, "fake-render.mjs");
  writeFileSync(
    script,
    `const args = process.argv.slice(2);
const cmd = args[0];
if (cmd === "info") {
  console.log(JSON.stringify({
    gpu_adapter: "Fake RTX 5080",
    backend: "Vulkan",
    device_type: "DiscreteGpu",
    is_discrete_gpu: true,
    nvenc_encoders: ["h264_nvenc", "hevc_nvenc", "av1_nvenc"]
  }));
  process.exit(0);
}
if (cmd === "validate") {
  if (args[1]?.includes("invalid")) {
    console.error("missing layer");
    process.exit(1);
  }
  console.log("OK: " + args[1] + " (essay 2 scenes, 1920x1080 @ 60 fps)");
  process.exit(0);
}
if (cmd === "snapshot") {
  console.log("Rendered 2 snapshots in 120 ms:");
  console.log("  out/snap_001_00s.png");
  console.log("  out/snap_003_50s.png");
  process.exit(0);
}
if (cmd === "render") {
  if (args.includes("--require-gpu") && args.includes("libx264")) {
    console.error("--require-gpu rejected software codec 'libx264'");
    process.exit(1);
  }
  const outIdx = args.indexOf("-o");
  const outPath = outIdx >= 0 ? args[outIdx + 1] : "out.mp4";
  console.log(JSON.stringify({
    output: outPath,
    gpu_name: "Fake RTX 5080",
    gpu_backend: "Vulkan",
    codec: "h264_nvenc",
    total_frames: 600,
    effective_fps: 320.5,
    require_gpu_passed: args.includes("--require-gpu")
  }));
  process.exit(0);
}
process.exit(2);
`,
  );
  if (process.platform === "win32") {
    const cmdFile = path.join(dir, "fake-render.cmd");
    writeFileSync(cmdFile, `@echo off\r\n"${process.execPath}" "${script}" %*\r\n`);
    return cmdFile;
  }
  return script;
}

async function pollUntilSettled(
  call: (name: string, args?: Record<string, unknown>) => Promise<{ isError: boolean; body: unknown; text: string }>,
  jobId: string,
): Promise<unknown> {
  for (let i = 0; i < 60; i += 1) {
    const res = await call("get_essay_render_status", { jobId });
    assert.equal(res.isError, false);
    const status = at(res.body, "status");
    if (status === "completed" || status === "failed") {
      return res.body;
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error("render job did not settle in time");
}

test("render GPU tools probe GPU, validate specs, take snapshots and run non-blocking GPU renders", async () => {
  const binDir = mkdtempSync(path.join(tmpdir(), "cliphub-fake-render-"));
  const fakeBin = writeFakeRenderCli(binDir);
  const harness = await openHarness({ renderBin: fakeBin });
  try {
    const info = await harness.call("render_gpu_info");
    assert.equal(info.isError, false, info.text);
    assert.equal(at(info.body, "gpu_adapter"), "Fake RTX 5080");
    assert.equal(at(info.body, "is_discrete_gpu"), true);

    const valid = await harness.call("validate_essay_spec", { specPath: "essay.json" });
    assert.equal(valid.isError, false, valid.text);
    assert.equal(at(valid.body, "ok"), true);

    const invalid = await harness.call("validate_essay_spec", { specPath: "invalid.json" });
    assert.equal(invalid.isError, true);

    const snaps = await harness.call("snapshot_essay", {
      specPath: "essay.json",
      at: [1.0, 3.5],
      outDir: path.join(harness.dir, "snaps"),
    });
    assert.equal(snaps.isError, false, snaps.text);
    assert.deepEqual(at(snaps.body, "files"), ["out/snap_001_00s.png", "out/snap_003_50s.png"]);

    const started = await harness.call("start_essay_render", {
      specPath: "essay.json",
      outputPath: path.join(harness.dir, "final.mp4"),
      workers: 2,
    });
    assert.equal(started.isError, false, started.text);
    assert.equal(at(started.body, "requireGpu"), true);
    const jobId = String(at(started.body, "jobId"));

    const done = await pollUntilSettled((n, a) => harness.call(n, a), jobId);
    assert.equal(at(done, "status"), "completed");
    assert.equal(at(done, "stats.effective_fps"), 320.5);
    assert.equal(at(done, "stats.require_gpu_passed"), true);

    const rejected = await harness.call("start_essay_render", {
      specPath: "essay.json",
      outputPath: path.join(harness.dir, "bad.mp4"),
      codec: "libx264",
      requireGpu: true,
    });
    const failedJob = await pollUntilSettled((n, a) => harness.call(n, a), String(at(rejected.body, "jobId")));
    assert.equal(at(failedJob, "status"), "failed");
    assert.match(String(at(failedJob, "error")), /require-gpu rejected/);
  } finally {
    await harness.close();
    rmSync(binDir, { recursive: true, force: true });
  }
});
