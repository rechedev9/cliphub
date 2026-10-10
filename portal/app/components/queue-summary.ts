import type { AdminJob, AdminOverview, AdminWorker } from "./api-types.ts";
import { formatDuration, formatRelative } from "./format.ts";
import { FAILURE_LABELS, reasonLabel, splitPauseReason, STAGE_WORDS } from "./labels.ts";

const DAY_MS = 24 * 60 * 60 * 1000;

export type Tone = "ok" | "live" | "warn" | "bad" | "off";

export interface WorkerStatus {
  tone: Tone;
  label: string;
}

export function userLabel(user: AdminJob["user"]): string {
  return user.name ?? user.email ?? user.id;
}

export function jobTitle(job: AdminJob): string {
  return job.title ?? job.demo.fileName ?? "Sin título";
}

// The one word the operator reads first; the most severe condition wins.
export function workerStatus(worker: AdminWorker, currentJob: AdminJob | null): WorkerStatus {
  if (worker.revoked) return { tone: "off", label: "Revocado" };
  if (!worker.online) return { tone: "bad", label: "Sin conexión" };
  if (worker.paused) return { tone: "bad", label: "En pausa" };
  if (worker.blocked !== null || worker.state === "blocked") return { tone: "warn", label: "Bloqueado" };
  // Right after a claim the worker still reports idle until its next heartbeat; the lease is what counts.
  if (worker.state === "busy" || currentJob?.status === "running") {
    const stage = currentJob?.stage ?? null;
    return { tone: "live", label: stage === null ? "Ocupado" : STAGE_WORDS[stage] };
  }
  return { tone: "ok", label: "Libre" };
}

// Three missed heartbeats: long enough that an idle report is not just the gap after a claim.
const FORGOTTEN_AFTER_MS = 45 * 1000;

// True when the worker keeps reporting idle while the portal still has a job leased to it,
// which is what a worker that restarted without its state looks like.
export function forgotItsJob(worker: AdminWorker, job: AdminJob | null, now: number): boolean {
  if (job === null || job.status !== "running" || !worker.online || worker.state !== "idle") return false;
  return job.startedAt !== null && now - job.startedAt > FORGOTTEN_AFTER_MS;
}

// True when the worker would take the next queued job right now.
export function canTakeWork(worker: AdminWorker): boolean {
  return worker.online && !worker.revoked && !worker.paused && worker.blocked === null && worker.state !== "blocked";
}

// What happens to the job a silent worker was holding, read from its lease.
export function leaseOutlook(job: AdminJob, now: number): string {
  if (job.status === "uploading") {
    return "Si no vuelve, el portal espera media hora más antes de devolver el trabajo a la cola.";
  }
  if (job.leaseExpiresAt === null || job.leaseExpiresAt <= now) {
    return "Su reserva ha caducado: el trabajo vuelve a la cola en unos segundos.";
  }
  return `Si no da señal en ${formatDuration((job.leaseExpiresAt - now) / 1000)}, el trabajo vuelve a la cola.`;
}

// Failed jobs that ended in the last 24 hours, newest first.
export function recentFailures(jobs: AdminJob[], now: number): AdminJob[] {
  return jobs
    .filter((job) => job.status === "failed" && (job.finishedAt ?? 0) >= now - DAY_MS)
    .sort((a, b) => (b.finishedAt ?? 0) - (a.finishedAt ?? 0));
}

// The first line of the worker's raw cause, short enough for one row.
export function causeHeadline(detail: string | null): string | null {
  const first = (detail ?? "").split(/\r?\n/).find((line) => line.trim() !== "");
  if (first === undefined) return null;
  const line = first.trim();
  return line.length > 160 ? `${line.slice(0, 159)}…` : line;
}

export interface SummaryCell {
  key: "worker" | "now" | "waiting" | "failures";
  label: string;
  value: string;
  detail: string | null;
  tone: Tone | "plain";
  href: string | null;
}

function lastSeenText(worker: AdminWorker, now: number): string {
  return worker.lastSeenAt === null ? "nunca se ha conectado" : `última señal ${formatRelative(worker.lastSeenAt, now)}`;
}

function workerReason(worker: AdminWorker, now: number): string {
  if (!worker.online) return lastSeenText(worker, now);
  if (worker.paused) return splitPauseReason(worker.pauseReason).label;
  if (worker.blocked !== null) return reasonLabel(worker.blocked.code);
  return lastSeenText(worker, now);
}

function workerCell(overview: AdminOverview): SummaryCell {
  const { workers, now } = overview;
  const cell = { key: "worker", href: null } as const;
  const [only] = workers;
  if (only === undefined) {
    return {
      ...cell,
      label: "Worker",
      value: "Ninguno",
      detail: "Nadie graba la cola. Crea uno en Workers.",
      tone: "bad",
      href: "/admin/workers",
    };
  }
  if (workers.length === 1) {
    const job = overview.running.find((candidate) => candidate.id === only.currentJobId) ?? null;
    const status = workerStatus(only, job);
    return { ...cell, label: only.name, value: status.label, detail: workerReason(only, now), tone: status.tone };
  }
  const ready = workers.filter(canTakeWork);
  const down = workers.filter((worker) => !canTakeWork(worker));
  return {
    ...cell,
    label: "Workers",
    value: `${ready.length} de ${workers.length} disponibles`,
    detail:
      down.length === 0
        ? null
        : down.map((worker) => `${worker.name}: ${workerStatus(worker, null).label.toLowerCase()}`).join(" · "),
    tone: ready.length === 0 ? "bad" : down.length === 0 ? "ok" : "warn",
  };
}

function nowCell(overview: AdminOverview): SummaryCell {
  const [job] = overview.running;
  const uploads = overview.uploading.length;
  const uploadText = uploads === 0 ? null : uploads === 1 ? "1 subiendo resultados" : `${uploads} subiendo resultados`;
  if (job === undefined) {
    return { key: "now", label: "Ahora", value: "Nada en curso", detail: uploadText, tone: "plain", href: null };
  }
  const word = job.stage === null ? "Preparando" : STAGE_WORDS[job.stage];
  const percent = job.progressPercent === null ? "" : ` ${job.progressPercent} %`;
  const who = `${jobTitle(job)} · ${userLabel(job.user)}`;
  const owner = overview.workers.find((worker) => worker.id === job.workerId);
  // A silent worker's progress is its last report, not what is happening now.
  if (owner === undefined || !owner.online) {
    return {
      key: "now",
      label: "Ahora",
      value: "Parado",
      detail: `${who} · se quedó en ${word.toLowerCase()}${percent}, sin señal del worker`,
      tone: "bad",
      href: `/admin/jobs/${job.id}`,
    };
  }
  return {
    key: "now",
    label: "Ahora",
    value: `${word}${percent}`,
    detail: uploadText === null ? who : `${who} · ${uploadText}`,
    tone: "live",
    href: `/admin/jobs/${job.id}`,
  };
}

function waitingCell(overview: AdminOverview): SummaryCell {
  const { queue } = overview;
  const cell = { key: "waiting", label: "En espera", href: null } as const;
  if (queue.length === 0) return { ...cell, value: "Nadie espera", detail: null, tone: "plain" };
  const people = new Set(queue.map((job) => job.user.id)).size;
  const oldest = Math.max(...queue.map((job) => job.waitedSeconds ?? 0));
  const stuck = !overview.workers.some(canTakeWork);
  const detail = `de ${people} ${people === 1 ? "persona" : "personas"} · el más antiguo espera ${formatDuration(oldest)}`;
  return {
    ...cell,
    value: queue.length === 1 ? "1 trabajo" : `${queue.length} trabajos`,
    detail: stuck ? `${detail} · ahora nadie los puede grabar` : detail,
    tone: stuck ? "warn" : "plain",
  };
}

function failuresCell(overview: AdminOverview, failures: AdminJob[]): SummaryCell {
  const count = overview.stats.failed24h;
  const cell = { key: "failures", label: "Fallos en 24 h" } as const;
  if (count === 0) return { ...cell, value: "Ninguno", detail: null, tone: "plain", href: null };
  const [last] = recentFailures(failures, overview.now);
  const code = last?.failure?.code;
  return {
    ...cell,
    value: String(count),
    detail:
      last === undefined || code === undefined
        ? null
        : `último: ${FAILURE_LABELS[code]} · ${formatRelative(last.finishedAt ?? overview.now, overview.now)}`,
    tone: "bad",
    href: "/admin/history?status=failed",
  };
}

// The four answers the operator needs first: is it alive, what is it doing, who waits, what failed.
export function queueSummary(overview: AdminOverview, failures: AdminJob[]): SummaryCell[] {
  return [workerCell(overview), nowCell(overview), waitingCell(overview), failuresCell(overview, failures)];
}
