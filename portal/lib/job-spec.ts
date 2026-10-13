import { isRecord, type CloudKind } from "./job-types.ts";

const MAX_SPEC_BYTES = 65536;

export interface JobWindow {
  id: string;
  tickStart: number;
  tickEnd: number;
}

export interface JobSpec {
  version: 1;
  targetSteamId: string;
  rules: Record<string, unknown>;
  capture: { tickrate: number; windows: JobWindow[] };
  generate: Record<string, unknown>;
  client?: Record<string, unknown>;
  fullDemo?: { options: Record<string, unknown> };
}

export type ParsedJobSpec =
  | { ok: true; spec: JobSpec; captureSeconds: number }
  | { ok: false; error: string };

interface SpecLimits {
  maxWindows: number;
  maxWindowSeconds: number;
  maxTotalSeconds: number;
}

// The full_demo caps are provisional: the kind is rejected until CLOUD_KINDS lists it.
const SPEC_LIMITS: Record<CloudKind, SpecLimits> = {
  short: { maxWindows: 60, maxWindowSeconds: 180, maxTotalSeconds: 900 },
  full_demo: { maxWindows: 80, maxWindowSeconds: 600, maxTotalSeconds: 7200 },
};

const STEAM_ID = /^[0-9]{17}$/;
const WINDOW_ID = /^[A-Za-z0-9_-]{1,40}$/;
const WINDOW_PADDING_SECONDS = 2;

function fail(error: string): ParsedJobSpec {
  return { ok: false, error };
}

function parseWindows(
  input: unknown,
): { ok: true; windows: JobWindow[] } | { ok: false; error: string } {
  if (!Array.isArray(input)) {
    return { ok: false, error: "capture.windows must be an array" };
  }
  const windows: JobWindow[] = [];
  const seen = new Set<string>();
  for (const entry of input) {
    if (!isRecord(entry)) {
      return { ok: false, error: "capture.windows entries must be objects" };
    }
    const { id, tickStart, tickEnd } = entry;
    if (typeof id !== "string" || !WINDOW_ID.test(id)) {
      return { ok: false, error: "window id is not valid" };
    }
    if (seen.has(id)) {
      return { ok: false, error: `window id ${id} is repeated` };
    }
    if (
      typeof tickStart !== "number" ||
      typeof tickEnd !== "number" ||
      !Number.isSafeInteger(tickStart) ||
      !Number.isSafeInteger(tickEnd) ||
      tickStart < 0 ||
      tickStart >= tickEnd
    ) {
      return { ok: false, error: `window ${id} has invalid ticks` };
    }
    seen.add(id);
    windows.push({ id, tickStart, tickEnd });
  }
  return { ok: true, windows };
}

function sameIds(windows: JobWindow[], segmentIds: unknown): boolean {
  return (
    Array.isArray(segmentIds) &&
    segmentIds.length === windows.length &&
    windows.every((window, index) => segmentIds[index] === window.id)
  );
}

function isObjectOrNull(value: unknown): boolean {
  return value === undefined || value === null || isRecord(value);
}

// Validates the job spec of contracts section D and returns the capture time it asks for.
export function parseJobSpec(input: unknown, kind: CloudKind): ParsedJobSpec {
  if (!isRecord(input)) return fail("spec must be an object");
  if (input.version !== 1) return fail("spec.version must be 1");

  const { targetSteamId, rules, capture, generate, client, fullDemo } = input;
  if (typeof targetSteamId !== "string" || !STEAM_ID.test(targetSteamId)) {
    return fail("targetSteamId must be a 17 digit SteamID64");
  }
  if (!isRecord(rules)) return fail("rules must be an object");
  if (!isRecord(capture)) return fail("capture must be an object");

  const { tickrate } = capture;
  if (
    typeof tickrate !== "number" ||
    !Number.isInteger(tickrate) ||
    tickrate < 16 ||
    tickrate > 256
  ) {
    return fail("capture.tickrate must be an integer from 16 to 256");
  }

  const parsed = parseWindows(capture.windows);
  if (!parsed.ok) return fail(parsed.error);
  const { windows } = parsed;
  const limits = SPEC_LIMITS[kind];
  if (windows.length < 1 || windows.length > limits.maxWindows) {
    return fail(`capture.windows must have 1 to ${limits.maxWindows} entries`);
  }

  let windowSeconds = 0;
  for (const window of windows) {
    const seconds = (window.tickEnd - window.tickStart) / tickrate;
    if (seconds > limits.maxWindowSeconds) {
      return fail(`window ${window.id} is longer than ${limits.maxWindowSeconds} s`);
    }
    windowSeconds += seconds;
  }
  if (windowSeconds > limits.maxTotalSeconds) {
    return fail(`the windows add up to more than ${limits.maxTotalSeconds} s`);
  }

  if (!isRecord(generate)) return fail("generate must be an object");
  const { preset } = generate;
  if (typeof preset !== "string" || preset.length < 1 || preset.length > 64) {
    return fail("generate.preset must be a string of 1 to 64 characters");
  }
  if (!isObjectOrNull(generate.music) || !isObjectOrNull(generate.edit)) {
    return fail("generate.music and generate.edit must be objects or null");
  }
  if (client !== undefined && !isRecord(client)) {
    return fail("client must be an object");
  }

  const spec: JobSpec = {
    version: 1,
    targetSteamId,
    rules,
    capture: { tickrate, windows },
    generate,
  };
  if (client !== undefined) spec.client = client;

  if (kind === "short") {
    if (!sameIds(windows, generate.segment_ids)) {
      return fail("generate.segment_ids must equal the window ids in order");
    }
  } else {
    if (!Array.isArray(generate.segment_ids) || generate.segment_ids.length > 0) {
      return fail("generate.segment_ids must be empty for a full demo");
    }
    if (isRecord(generate.edit) && "full_demo" in generate.edit) {
      return fail("generate.edit must not carry full_demo");
    }
    if (!isRecord(fullDemo) || !isRecord(fullDemo.options)) {
      return fail("fullDemo.options must be an object");
    }
    spec.fullDemo = { options: fullDemo.options };
  }

  if (Buffer.byteLength(JSON.stringify(spec), "utf8") > MAX_SPEC_BYTES) {
    return fail(`spec is larger than ${MAX_SPEC_BYTES} bytes`);
  }

  return {
    ok: true,
    spec,
    captureSeconds: windowSeconds + WINDOW_PADDING_SECONDS * windows.length,
  };
}

// Reads a stored spec column back. It was validated when the job was created.
export function readStoredSpec(raw: string | null): Record<string, unknown> | null {
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}
