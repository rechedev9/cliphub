import {
  NOW,
  getJob,
  makeQueuedJob,
  makeUser,
  makeWorker,
  resetDatabase,
} from "./test-support.ts";

import { beforeEach, test } from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";

import { eq } from "drizzle-orm";

import { db } from "../db/client.ts";
import { requestArtifacts, requests } from "../db/schema.ts";
import {
  artifactResponse,
  completeArtifact,
  initArtifact,
  markArtifactReceived,
  readyArtifact,
  writeArtifactPart,
} from "./artifact-store.ts";
import { claimNext, completeJob, enterUploading, type WorkerRow } from "./queue-store.ts";
import type { ArtifactInitBody } from "./worker-protocol.ts";

const PART = 1024;
// An mp4 starts with a box size and the box type "ftyp".
const VIDEO = Buffer.concat([Buffer.from([0, 0, 0, 24]), Buffer.from("ftypisom"), Buffer.alloc(2488, 3)]);

function sha256(bytes: Buffer): string {
  return createHash("sha256").update(bytes).digest("hex");
}

function describe(bytes: Buffer, overrides: Partial<ArtifactInitBody> = {}): ArtifactInitBody {
  return {
    name: "short-01.mp4",
    kind: "video",
    variant: "viral-60-clean",
    sizeBytes: bytes.length,
    sha256: sha256(bytes),
    ...overrides,
  };
}

function partRequest(bytes: Buffer): Request {
  return new Request("http://portal.test/part", {
    method: "PUT",
    body: new Uint8Array(bytes),
    headers: { "content-length": String(bytes.length) },
  });
}

function partOf(bytes: Buffer, part: number): Buffer {
  return bytes.subarray((part - 1) * PART, part * PART);
}

interface Setup {
  worker: WorkerRow;
  jobId: string;
  attempt: number;
}

async function uploadingJob(): Promise<Setup> {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("luis") });
  await claimNext({ worker, kinds: ["short"], now: NOW });
  const attempt = 1;
  await enterUploading({
    worker,
    jobId,
    attempt,
    body: { machineSeconds: 300, localJobId: null },
    now: NOW,
  });
  return { worker, jobId, attempt };
}

async function begin(setup: Setup, body: ArtifactInitBody) {
  const result = await initArtifact({ ...setup, body, now: NOW });
  if (!result.ok) throw new Error(`init refused: ${result.code}`);
  return result;
}

interface PartToSend {
  artifactId: string;
  part: number;
  bytes: Buffer;
}

async function send(setup: Setup, options: PartToSend) {
  return writeArtifactPart({
    ...setup,
    artifactId: options.artifactId,
    part: options.part,
    request: partRequest(options.bytes),
  });
}

function refusal(result: { ok: true } | { ok: false; status: number; code: string }) {
  return result.ok ? null : [result.status, result.code];
}

beforeEach(async () => {
  await resetDatabase();
  process.env.ARTIFACT_PART_BYTES = String(PART);
});

test("a file uploaded in parts, in any order, is assembled, verified and becomes downloadable", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  assert.deepEqual(
    { partSize: started.partSize, partCount: started.partCount, receivedParts: started.receivedParts },
    { partSize: PART, partCount: 3, receivedParts: [] },
  );

  for (const part of [3, 1, 2]) {
    const sent = await send(setup, { artifactId: started.artifactId, part, bytes: partOf(VIDEO, part) });
    assert.equal(sent.ok, true);
  }
  assert.deepEqual(await completeArtifact({ ...setup, artifactId: started.artifactId, now: NOW }), { ok: true });

  const artifact = await readyArtifact(setup.jobId, started.artifactId);
  assert.deepEqual(await readFile(artifact?.path ?? ""), VIDEO);
  assert.equal(await completeJob({ ...setup, now: NOW }), "ok");
  assert.equal((await getJob(setup.jobId)).status, "done");
});

test("init is idempotent and returns the parts already received, so an upload resumes", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  await send(setup, { artifactId: started.artifactId, part: 1, bytes: partOf(VIDEO, 1) });

  const resumed = await begin(setup, describe(VIDEO));
  assert.equal(resumed.artifactId, started.artifactId);
  assert.deepEqual(resumed.receivedParts, [1]);
});

test("init with other content under the same name starts that file over", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  await send(setup, { artifactId: started.artifactId, part: 1, bytes: partOf(VIDEO, 1) });

  const other = Buffer.concat([VIDEO, Buffer.from("more")]);
  const restarted = await begin(setup, describe(other));
  assert.equal(restarted.artifactId, started.artifactId);
  assert.deepEqual(restarted.receivedParts, []);
  assert.equal(restarted.partCount, 3);
});

test("completing with parts missing lists them and keeps the file unfinished", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  await send(setup, { artifactId: started.artifactId, part: 2, bytes: partOf(VIDEO, 2) });

  const result = await completeArtifact({ ...setup, artifactId: started.artifactId, now: NOW });
  assert.deepEqual(refusal(result), [409, "parts_missing"]);
  if (!result.ok) assert.deepEqual(result.extra, { missing: [1, 3] });
  assert.equal(await readyArtifact(setup.jobId, started.artifactId), null);
  assert.equal(await completeJob({ ...setup, now: NOW }), "no_artifacts");
});

test("a part with the wrong length or number is refused and not recorded", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));

  const short = await send(setup, { artifactId: started.artifactId, part: 1, bytes: partOf(VIDEO, 1).subarray(0, 100) });
  const beyond = await send(setup, { artifactId: started.artifactId, part: 4, bytes: partOf(VIDEO, 1) });
  const lastTooLong = await send(setup, { artifactId: started.artifactId, part: 3, bytes: partOf(VIDEO, 1) });
  assert.deepEqual(refusal(short), [400, "bad_part_length"]);
  assert.deepEqual(refusal(beyond), [400, "bad_part_length"]);
  assert.deepEqual(refusal(lastTooLong), [400, "bad_part_length"]);
  assert.deepEqual((await begin(setup, describe(VIDEO))).receivedParts, []);
});

test("a part whose body is shorter than its Content-Length is not recorded", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  const lying = new Request("http://portal.test/part", {
    method: "PUT",
    body: new Uint8Array(partOf(VIDEO, 1).subarray(0, 10)),
    headers: { "content-length": String(PART) },
  });

  const result = await writeArtifactPart({ ...setup, artifactId: started.artifactId, part: 1, request: lying });
  assert.deepEqual(refusal(result), [400, "bad_part_length"]);
  assert.deepEqual((await begin(setup, describe(VIDEO))).receivedParts, []);
});

test("bytes that do not match the declared hash are rejected and the upload starts over", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  const corrupted = Buffer.from(VIDEO);
  corrupted[2000] = 99;
  for (const part of [1, 2, 3]) {
    await send(setup, { artifactId: started.artifactId, part, bytes: partOf(corrupted, part) });
  }

  const result = await completeArtifact({ ...setup, artifactId: started.artifactId, now: NOW });
  assert.deepEqual(refusal(result), [422, "sha256_mismatch"]);
  assert.deepEqual((await begin(setup, describe(VIDEO))).receivedParts, []);
});

test("a file declared as a video that is not an mp4 is rejected", async () => {
  const setup = await uploadingJob();
  const notVideo = Buffer.alloc(1500, 65);
  const started = await begin(setup, describe(notVideo));
  for (const part of [1, 2]) {
    await send(setup, { artifactId: started.artifactId, part, bytes: partOf(notVideo, part) });
  }
  const result = await completeArtifact({ ...setup, artifactId: started.artifactId, now: NOW });
  assert.deepEqual(refusal(result), [422, "not_a_video"]);
});

test("only the worker that holds the job in uploading can send files for it", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  const intruder = await makeWorker("intruder");
  const asIntruder = { ...setup, worker: intruder };

  const init = await initArtifact({ ...asIntruder, body: describe(VIDEO), now: NOW });
  const part = await send(asIntruder, { artifactId: started.artifactId, part: 1, bytes: partOf(VIDEO, 1) });
  const complete = await completeArtifact({ ...asIntruder, artifactId: started.artifactId, now: NOW });
  assert.deepEqual(refusal(init), [409, "lease_lost"]);
  assert.deepEqual(refusal(part), [409, "lease_lost"]);
  assert.deepEqual(refusal(complete), [409, "lease_lost"]);
});

test("files are refused while the job is still capturing", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("luis") });
  await claimNext({ worker, kinds: ["short"], now: NOW });
  const result = await initArtifact({ worker, jobId, attempt: 1, body: describe(VIDEO), now: NOW });
  assert.deepEqual(refusal(result), [409, "lease_lost"]);
});

test("an older attempt of the same job, on the same worker, cannot touch the files of the new one", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  // The job comes back to the same worker as attempt 2 and reaches uploading again.
  await db.update(requests).set({ attempt: 2 }).where(eq(requests.id, setup.jobId));

  const init = await initArtifact({ ...setup, body: describe(VIDEO), now: NOW });
  const part = await send(setup, { artifactId: started.artifactId, part: 1, bytes: partOf(VIDEO, 1) });
  const complete = await completeArtifact({ ...setup, artifactId: started.artifactId, now: NOW });
  assert.deepEqual(refusal(init), [409, "lease_lost"]);
  assert.deepEqual(refusal(part), [409, "lease_lost"]);
  assert.deepEqual(refusal(complete), [409, "lease_lost"]);
  assert.equal(await completeJob({ ...setup, now: NOW }), "lease_lost");

  const current = { ...setup, attempt: 2 };
  assert.deepEqual(refusal(await send(current, { artifactId: started.artifactId, part: 1, bytes: partOf(VIDEO, 1) })), null);
});

test("a file over the size cap for the kind is refused before any byte is sent", async () => {
  const setup = await uploadingJob();
  const result = await initArtifact({
    ...setup,
    body: describe(VIDEO, { sizeBytes: 1024 * 1024 * 1024 + 1 }),
    now: NOW,
  });
  assert.deepEqual(refusal(result), [413, "artifact_too_large"]);
});

test("a job takes at most 40 files", async () => {
  const setup = await uploadingJob();
  for (let index = 0; index < 40; index += 1) {
    await begin(setup, describe(VIDEO, { name: `short-${index}.mp4` }));
  }
  const result = await initArtifact({ ...setup, body: describe(VIDEO, { name: "one-more.mp4" }), now: NOW });
  assert.deepEqual(refusal(result), [409, "too_many_artifacts"]);
});

test("the stored file path never contains the name the worker sent", async () => {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO, { name: "evil..name.mp4" }));
  const [row] = await db.select().from(requestArtifacts).where(eq(requestArtifacts.id, started.artifactId));
  assert.equal(row?.path.includes("evil"), false);
  assert.equal(row?.path.endsWith(`${started.artifactId}.bin`), true);
});

async function readyVideo(): Promise<{ jobId: string; artifactId: string }> {
  const setup = await uploadingJob();
  const started = await begin(setup, describe(VIDEO));
  for (const part of [1, 2, 3]) {
    await send(setup, { artifactId: started.artifactId, part, bytes: partOf(VIDEO, part) });
  }
  await completeArtifact({ ...setup, artifactId: started.artifactId, now: NOW });
  return { jobId: setup.jobId, artifactId: started.artifactId };
}

test("a download can resume from a byte offset and carries the hash as its ETag", async () => {
  const { jobId, artifactId } = await readyVideo();
  const artifact = await readyArtifact(jobId, artifactId);
  if (!artifact) throw new Error("artifact not ready");

  const whole = await artifactResponse({ request: new Request("http://portal.test/a"), artifact, attachment: true });
  assert.equal(whole.status, 200);
  assert.equal(whole.headers.get("content-length"), String(VIDEO.length));
  assert.equal(whole.headers.get("accept-ranges"), "bytes");
  assert.equal(whole.headers.get("etag"), `"${sha256(VIDEO)}"`);
  assert.equal(whole.headers.get("content-type"), "video/mp4");
  assert.equal(whole.headers.get("content-disposition"), 'attachment; filename="short-01.mp4"');
  assert.deepEqual(Buffer.from(await whole.arrayBuffer()), VIDEO);

  const ranged = new Request("http://portal.test/a", { headers: { range: "bytes=1000-" } });
  const rest = await artifactResponse({ request: ranged, artifact, attachment: true });
  assert.equal(rest.status, 206);
  assert.equal(rest.headers.get("content-range"), `bytes 1000-${VIDEO.length - 1}/${VIDEO.length}`);
  assert.deepEqual(Buffer.from(await rest.arrayBuffer()), VIDEO.subarray(1000));

  const slice = new Request("http://portal.test/a", { headers: { range: "bytes=4-7" } });
  const box = await artifactResponse({ request: slice, artifact, attachment: false });
  assert.equal(Buffer.from(await box.arrayBuffer()).toString("latin1"), "ftyp");
  assert.equal(box.headers.get("content-disposition"), null);

  const past = new Request("http://portal.test/a", { headers: { range: `bytes=${VIDEO.length}-` } });
  const refused = await artifactResponse({ request: past, artifact, attachment: true });
  assert.equal(refused.status, 416);
  assert.equal(refused.headers.get("content-range"), `bytes */${VIDEO.length}`);
});

test("the receipt mark is set once and an artifact of another job is not found", async () => {
  const { jobId, artifactId } = await readyVideo();
  const artifact = await readyArtifact(jobId, artifactId);
  if (!artifact) throw new Error("artifact not ready");

  await markArtifactReceived(artifact, NOW + 1000);
  const marked = await readyArtifact(jobId, artifactId);
  if (!marked) throw new Error("artifact not ready");
  await markArtifactReceived(marked, NOW + 9000);
  assert.equal((await readyArtifact(jobId, artifactId))?.receivedAt, NOW + 1000);

  const otherJob = await makeQueuedJob({ userId: await makeUser("other") });
  assert.equal(await readyArtifact(otherJob, artifactId), null);
});
