import assert from "node:assert/strict";
import { test } from "node:test";

import type { ToolResult } from "../bridge.ts";
import type { Json } from "../json.ts";
import { describeVideos, followResult, formatElapsed, keepCalls, readProgress, videoNames } from "./progress.ts";

const JOB = "6f1d2c1e-8a4b-4c55-9e0e-2b7d4f6a9c10";
const ok = (structured: Json): ToolResult => ({ structured, text: "", isError: false });

test("a Full Demo render keeps following wait_for_job with the variant from the call", () => {
  const p = readProgress(ok({ job_id: JOB, status: "recorded", running: true, percent: 63, stage: "Montando corte 17 de 18" }), {
    id: JOB,
    variant: "gameplay-pov-60",
  });
  assert.equal(p?.title, "Renderizando");
  assert.equal(p?.detail, "Montando corte 17 de 18");
  assert.equal(p?.percent, 63);
  assert.deepEqual(p?.follow, { tool: "wait_for_job", args: { id: JOB, wait_seconds: 5, variant: "gameplay-pov-60" } });
});

test("a started capture follows its own variant before any wait_for_job", () => {
  const p = readProgress(ok({ summary: "Grabar…", job_id: JOB, status: "queued", variant: "short-9x16" }), {});
  assert.equal(p?.phase, "queued");
  assert.equal(p?.follow?.args.variant, "short-9x16");
});

test("polls after a started capture keep its variant, which wait_for_job does not echo", () => {
  const start = readProgress(ok({ summary: "Grabar", job_id: JOB, status: "queued", variant: "short-9x16" }), { job_id: JOB });
  const follow = start?.follow;
  assert.ok(follow);
  const recording = followResult(follow, ok({ job_id: JOB, status: "recording", running: true, percent: 30 }));
  assert.equal(recording?.follow?.args.variant, "short-9x16");
  const done = followResult(follow, ok({ job_id: JOB, status: "recorded", running: false, renders: [{ variant: "short-9x16", status: "ready" }] }));
  assert.equal(done?.variant, "short-9x16");
});

test("a capture waiting for approval shows nothing", () => {
  assert.equal(readProgress(ok({ needs_approval: true, approval_code: "abc", summary: "…" }), {}), null);
});

test("each job status maps to its Spanish title", () => {
  const titles = ["queued", "parsing", "recording"].map((status) => readProgress(ok({ job_id: JOB, status, running: true }), {})?.title);
  assert.deepEqual(titles, ["En cola", "Analizando", "Grabando en CS2"]);
  const queuedCapture = readProgress(ok({ job_id: JOB, status: "recorded", running: true, stage: "capture queued" }), {});
  assert.equal(queuedCapture?.phase, "queued");
  assert.equal(queuedCapture?.detail, "Esperando a que CS2 quede libre");
});

test("a finished render asks list_videos for the names, a finished parse does not", () => {
  const rendered = readProgress(ok({ job_id: JOB, status: "recorded", running: false, renders: [{ variant: "v", status: "ready" }] }), {});
  assert.equal(rendered?.title, "Listo");
  assert.equal(rendered?.follow, null);
  assert.deepEqual(rendered?.videos, { tool: "list_videos", args: { job_id: JOB } });
  const parsed = readProgress(ok({ job_id: JOB, status: "parsed", running: false }), {});
  assert.equal(parsed?.videos, null);
});

test("a failed job shows its reason and stops", () => {
  const p = readProgress(ok({ job_id: JOB, status: "recorded", running: false, failure_code: "render_failed", failure_reason: "ffmpeg salió con 1" }), {});
  assert.equal(p?.title, "Error");
  assert.equal(p?.detail, "ffmpeg salió con 1");
  assert.equal(p?.follow, null);
});

test("a demo import reads as Importando, then Analizando, then Listo", () => {
  const states = ["queued", "parsing", "ready"].map((state) => readProgress(ok({ demo_id: JOB, state }), {})?.title);
  assert.deepEqual(states, ["Importando", "Analizando", "Listo"]);
});

test("a stream render translates the video counter", () => {
  const p = readProgress(ok({ stream_id: JOB, variant: "vertical", status: "rendering", running: true, percent: 40, progress: "video 2 of 3" }), {});
  assert.equal(p?.detail, "Vídeo 2 de 3");
  assert.equal(p?.follow?.tool, "wait_for_stream_render");
});

test("a stream render that was never started is not shown as done", () => {
  for (const [variant, status] of [["vertical", "ready"], ["summary", "idle"]]) {
    const p = readProgress(ok({ stream_id: JOB, variant, status, running: false }), {});
    assert.equal(p?.phase, "failed");
    assert.equal(p?.videos, null);
  }
});

test("keepCalls keeps the follow call identity across equal refreshes", () => {
  const read = (percent: number) => readProgress(ok({ job_id: JOB, status: "recording", running: true, percent }), {});
  const first = read(10);
  const next = keepCalls(first, read(20));
  assert.equal(next?.percent, 20);
  assert.equal(next?.follow, first?.follow);
});

test("video names come from the followed variant only", () => {
  const result = ok({
    videos: [
      { variant: "gameplay-pov-60", name: "mirage_full.mp4" },
      { variant: "short-9x16", name: "short.mp4" },
    ],
  });
  assert.deepEqual(videoNames(result, "gameplay-pov-60"), ["mirage_full.mp4"]);
  assert.equal(describeVideos(["a", "b", "c", "d"]), "4 vídeos: a, b, c y 1 más");
});

test("elapsed time reads as minutes and seconds, with hours past one", () => {
  assert.equal(formatElapsed(72_000), "1:12");
  assert.equal(formatElapsed(3_723_000), "1:02:03");
});
