import { execFile, spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { existsSync } from "node:fs";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

const REPO_ROOT = path.resolve(import.meta.dirname, "..", "..", "..");
const DEFAULT_RELEASE_BIN = path.join(
  REPO_ROOT,
  "tools",
  "render",
  "target",
  "release",
  process.platform === "win32" ? "cliphub-render.exe" : "cliphub-render",
);

export type GpuProbeInfo = {
  gpu_adapter: string;
  backend: string;
  device_type: string;
  is_discrete_gpu: boolean;
  nvenc_encoders: string[];
};

export type RenderJobState = {
  jobId: string;
  status: "running" | "completed" | "failed";
  pid: number | null;
  specPath: string;
  outputPath: string;
  requireGpu: boolean;
  startedAt: string;
  finishedAt: string | null;
  stats: Record<string, unknown> | null;
  error: string | null;
};

export type StartRenderInput = {
  specPath: string;
  outputPath: string;
  codec?: string;
  preset?: string;
  cq?: number;
  workers?: number;
  lufs?: number;
  requireGpu?: boolean;
  cudaDecode?: boolean;
};

export type SnapshotInput = {
  specPath: string;
  at: number[];
  outDir: string;
  scene?: string;
  requireGpu?: boolean;
};

export function resolveRenderBinary(configuredBin: string | null): string {
  if (configuredBin !== null && configuredBin.trim().length > 0) {
    return configuredBin;
  }
  if (existsSync(DEFAULT_RELEASE_BIN)) {
    return DEFAULT_RELEASE_BIN;
  }
  return process.platform === "win32" ? "cliphub-render.exe" : "cliphub-render";
}

function buildRenderArgs(input: StartRenderInput): string[] {
  const args = ["render", input.specPath, "-o", input.outputPath, "--stats-json"];
  if (input.codec !== undefined) args.push("--codec", input.codec);
  if (input.preset !== undefined) args.push("--preset", input.preset);
  if (input.cq !== undefined) args.push("--cq", String(input.cq));
  if (input.workers !== undefined) args.push("--workers", String(input.workers));
  if (input.lufs !== undefined) args.push("--lufs", String(input.lufs));
  if (input.requireGpu !== false) args.push("--require-gpu");
  if (input.cudaDecode === true) args.push("--cuda-decode");
  return args;
}

function needsShell(bin: string): boolean {
  return process.platform === "win32" && /\.(cmd|bat)$/i.test(bin);
}

export async function probeRenderGpu(bin: string, timeoutMs = 15_000): Promise<GpuProbeInfo> {
  const { stdout } = await execFileAsync(bin, ["info", "--json"], {
    timeout: timeoutMs,
    windowsHide: true,
    shell: needsShell(bin),
  });
  return JSON.parse(stdout) as GpuProbeInfo;
}

export async function validateEssaySpec(bin: string, specPath: string, timeoutMs = 15_000): Promise<{ ok: true; summary: string }> {
  const { stdout } = await execFileAsync(bin, ["validate", specPath], {
    timeout: timeoutMs,
    windowsHide: true,
    shell: needsShell(bin),
  });
  return { ok: true, summary: stdout.trim() };
}

export async function runEssaySnapshot(
  bin: string,
  input: SnapshotInput,
  timeoutMs = 60_000,
): Promise<{ outputDir: string; files: string[]; rawOutput: string }> {
  const args = ["snapshot", input.specPath, "--at", input.at.join(","), "-o", input.outDir];
  if (input.scene !== undefined) args.push("--scene", input.scene);
  if (input.requireGpu !== false) args.push("--require-gpu");
  const { stdout } = await execFileAsync(bin, args, {
    timeout: timeoutMs,
    windowsHide: true,
    shell: needsShell(bin),
  });
  const files = stdout
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.endsWith(".png"));
  return { outputDir: input.outDir, files, rawOutput: stdout.trim() };
}

type TrackedJob = {
  state: RenderJobState;
  child: ChildProcess;
};

export class RenderJobManager {
  readonly #jobs = new Map<string, TrackedJob>();

  start(bin: string, input: StartRenderInput): RenderJobState {
    const jobId = randomUUID();
    const args = buildRenderArgs(input);
    const child = spawn(bin, args, {
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
      shell: needsShell(bin),
    });
    let stdout = "";
    let stderr = "";

    const state: RenderJobState = {
      jobId,
      status: "running",
      pid: child.pid ?? null,
      specPath: input.specPath,
      outputPath: input.outputPath,
      requireGpu: input.requireGpu !== false,
      startedAt: new Date().toISOString(),
      finishedAt: null,
      stats: null,
      error: null,
    };

    const entry: TrackedJob = { state, child };
    this.#jobs.set(jobId, entry);

    child.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString("utf8");
    });
    child.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString("utf8");
    });
    child.on("error", (err) => {
      state.status = "failed";
      state.finishedAt = new Date().toISOString();
      state.error = err.message;
    });
    child.on("close", (code) => {
      state.finishedAt = new Date().toISOString();
      if (code === 0) {
        try {
          state.stats = JSON.parse(stdout) as Record<string, unknown>;
          state.status = "completed";
        } catch (parseErr) {
          state.status = "failed";
          state.error = `render exited 0 but stdout was not valid stats JSON: ${String(parseErr)}`;
        }
      } else {
        state.status = "failed";
        state.error = stderr.trim() || stdout.trim() || `render exited with code ${String(code)}`;
      }
    });

    return { ...state };
  }

  get(jobId: string): RenderJobState | null {
    const entry = this.#jobs.get(jobId);
    return entry === undefined ? null : { ...entry.state };
  }

  list(): RenderJobState[] {
    return Array.from(this.#jobs.values(), (entry) => ({ ...entry.state }));
  }

  closeAll(): void {
    for (const entry of this.#jobs.values()) {
      if (entry.state.status === "running") {
        entry.child.kill();
      }
    }
  }
}
