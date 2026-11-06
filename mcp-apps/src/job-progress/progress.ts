import type { ToolResult } from "../bridge.ts";
import { type Json, list, num, text } from "../json.ts";

export type Phase = "queued" | "importing" | "analysing" | "recording" | "rendering" | "done" | "failed";

export type ToolCall = { tool: string; args: Json };

export type Progress = {
  phase: Phase;
  title: string;
  detail: string;
  // null while the server reports no percent: the bar runs indeterminate.
  percent: number | null;
  // The call that refreshes a running job, and the one that names its videos once done.
  follow: ToolCall | null;
  videos: ToolCall | null;
  variant: string;
};

// Short enough that the bar moves often; the server polls Studio every 2 s inside it.
export const FOLLOW_WAIT_SECONDS = 5;
const MAX_DETAIL = 200;

const TITLES: Record<Phase, string> = {
  queued: "En cola",
  importing: "Importando",
  analysing: "Analizando",
  recording: "Grabando en CS2",
  rendering: "Renderizando",
  done: "Listo",
  failed: "Error",
};

// Stages zv-mcp writes itself; Studio's own stages are already in Spanish.
const STAGE_TEXT: Record<string, string> = {
  "capture queued": "Esperando a que CS2 quede libre",
  "render queued": "En cola para renderizar",
  rendering: "",
};

function clip(value: string): string {
  return value.length > MAX_DETAIL ? value.slice(0, MAX_DETAIL - 1) + "…" : value;
}

function stageText(stage: string): string {
  const fixed = STAGE_TEXT[stage];
  if (fixed !== undefined) return fixed;
  const video = /^video (\d+) of (\d+)$/.exec(stage);
  return video ? `Vídeo ${video[1]} de ${video[2]}` : stage;
}

function view(phase: Phase, fields: Partial<Progress>): Progress {
  return { phase, title: TITLES[phase], detail: "", percent: null, follow: null, videos: null, variant: "", ...fields };
}

function runningPercent(value: unknown): number | null {
  const p = num(value);
  return p !== null && p > 0 ? Math.min(Math.round(p), 99) : null;
}

function readDemo(s: Json): Progress {
  const id = text(s.demo_id);
  const follow = { tool: "wait_for_demo", args: { id, wait_seconds: FOLLOW_WAIT_SECONDS } };
  switch (text(s.state)) {
    case "ready":
      return view("done", { detail: "La demo está analizada", percent: 100 });
    case "failed":
      return view("failed", { detail: clip(text(s.error)) || "Studio no pudo analizar la demo" });
    case "parsing":
      return view("analysing", { detail: "Leyendo la demo", follow });
    default:
      return view("importing", { detail: "Esperando turno en Studio", follow });
  }
}

function readStream(s: Json, input: Json): Progress {
  const id = text(s.stream_id);
  const variant = text(s.variant) || text(input.variant);
  const status = text(s.status);
  const isStart = "summary" in s;
  const videos = { tool: "list_stream_videos", args: { id } };
  if (status === "failed" || text(s.error)) return view("failed", { detail: clip(text(s.error)) || "El render del stream falló", variant });
  // ready (a render) and idle (the summary) mean this variant was never started.
  if (!isStart && (status === "ready" || status === "idle")) return view("failed", { detail: "Este render no se ha iniciado", variant });
  if (!isStart && s.running !== true) return view("done", { percent: 100, videos, variant });
  const follow = { tool: "wait_for_stream_render", args: { id, variant, wait_seconds: FOLLOW_WAIT_SECONDS } };
  if (status === "queued") return view("queued", { detail: "El render empieza en cuanto Studio quede libre", follow, variant });
  return view("rendering", { detail: stageText(text(s.progress)), percent: runningPercent(s.percent), follow, variant });
}

function runningJobPhase(status: string, stage: string): Phase {
  switch (status) {
    case "queued":
      return "queued";
    case "parsing":
    case "scanning":
      return "analysing";
    case "recording":
      return "recording";
    case "recorded":
      return stage === "capture queued" ? "queued" : "rendering";
    default:
      return "rendering";
  }
}

function readJob(s: Json, input: Json): Progress {
  const id = text(s.job_id);
  const variant = text(s.variant) || text(input.variant);
  const status = text(s.status);
  const failure = text(s.failure_code);
  if (failure || status === "failed") {
    return view("failed", { detail: clip(text(s.failure_reason)) || failure || "El trabajo falló", variant });
  }
  // A start result has no running flag: the job was just queued.
  const running = "summary" in s || s.running === true;
  if (running) {
    const stage = text(s.stage);
    const phase = runningJobPhase(status, stage);
    const detail = stageText(stage) || (phase === "analysing" ? "Buscando las jugadas del jugador" : "");
    const args: Json = { id, wait_seconds: FOLLOW_WAIT_SECONDS };
    if (variant) args.variant = variant;
    return view(phase, { detail, percent: runningPercent(s.percent), follow: { tool: "wait_for_job", args }, variant });
  }
  if (status === "parsed" || status === "scanned") return view("done", { detail: "Las jugadas están listas", percent: 100 });
  const rendered = variant !== "" || list(s.renders).length > 0;
  return view("done", { percent: 100, variant, videos: rendered ? { tool: "list_videos", args: { job_id: id } } : null });
}

// readProgress turns a zv-mcp result into what the view shows; null hides the view,
// as for a capture that still waits for the user's approval.
export function readProgress(result: ToolResult, input: Json): Progress | null {
  const s = result.structured;
  if (result.isError) return view("failed", { detail: clip(result.text) || "Studio devolvió un error" });
  if (s.needs_approval === true) return null;
  if (text(s.demo_id) && "state" in s) return readDemo(s);
  if (text(s.stream_id)) return readStream(s, input);
  if (text(s.job_id)) return readJob(s, input);
  return null;
}

// followResult reads a poll against the call that made it: wait_for_job does not echo
// the variant, and only the call's own args still hold it.
export function followResult(call: ToolCall, result: ToolResult): Progress | null {
  return readProgress(result, call.args);
}

function sameCall(a: ToolCall | null, b: ToolCall | null): boolean {
  return a !== null && b !== null && a.tool === b.tool && JSON.stringify(a.args) === JSON.stringify(b.args);
}

// keepCalls reuses the previous calls when a refresh repeats them, so effects keyed on
// them keep running instead of restarting on every poll.
export function keepCalls(prev: Progress | null, next: Progress | null): Progress | null {
  if (!prev || !next) return next;
  return {
    ...next,
    follow: sameCall(prev.follow, next.follow) ? prev.follow : next.follow,
    videos: sameCall(prev.videos, next.videos) ? prev.videos : next.videos,
  };
}

// videoNames reads list_videos or list_stream_videos for the variant this view followed.
export function videoNames(result: ToolResult, variant: string): string[] {
  return list(result.structured.videos)
    .filter((v) => !variant || text(v.variant) === variant)
    .map((v) => text(v.name) || text(v.title))
    .filter((name) => name !== "");
}

export function describeVideos(names: string[]): string {
  if (names.length <= 1) return names[0] ?? "";
  const shown = names.slice(0, 3).join(", ");
  return names.length > 3 ? `${names.length} vídeos: ${shown} y ${names.length - 3} más` : `${names.length} vídeos: ${shown}`;
}

export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = String(total % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`;
}
