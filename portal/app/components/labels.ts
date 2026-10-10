import { CLOUD_ACCESS, FAILURE_CODES, JOB_KINDS, STAGES } from "./api-types.ts";
import type { CloudAccess, FailureCode, JobKind, JobStatus, Stage } from "./api-types.ts";
import { formatBytes, formatClock, formatDuration } from "./format.ts";

export const STATUS_LABELS: Record<JobStatus, string> = {
  awaiting_demo: "Esperando demo",
  queued: "En cola",
  running: "En curso",
  uploading: "Subiendo resultados",
  done: "Listo",
  failed: "Error",
  canceled: "Cancelado",
  pending: "Pendiente (antiguo)",
  approved: "Aprobada (antiguo)",
  processing: "En proceso (antiguo)",
  rejected: "Rechazada",
};

export const KIND_LABELS: Record<JobKind, string> = {
  manual: "Manual",
  short: "Short",
  full_demo: "Full Demo",
};

export const STAGE_LABELS: Record<Stage, string> = {
  downloading: "Descargando la demo",
  parsing: "Analizando la demo",
  capturing: "Grabando",
  rendering: "Montando el vídeo",
};

// One word for a running job: only the capture itself reads as recording.
export const STAGE_WORDS: Record<Stage, string> = {
  downloading: "Preparando",
  parsing: "Preparando",
  capturing: "Grabando",
  rendering: "Montando",
};

export const ACCESS_LABELS: Record<CloudAccess, string> = {
  pending: "Pendiente",
  allowed: "Permitido",
  blocked: "Bloqueado",
};

// Short operator-facing names; the user-facing sentence comes from the worker.
export const FAILURE_LABELS: Record<FailureCode, string> = {
  capture_flake: "Fallo puntual de captura",
  interrupted: "Captura interrumpida",
  worker_lost: "Worker perdido",
  demo_download_failed: "No se pudo descargar la demo",
  internal: "Error interno",
  demo_incompatible: "Demo incompatible",
  unplayable_start: "La demo no arranca",
  target_not_found: "Jugador no encontrado",
  spec_mismatch: "El plan no coincide con la demo",
  invalid_spec: "Plan no válido",
  render_failed: "Fallo de montaje",
  job_failed: "Fallo del trabajo",
  timeout: "Tiempo agotado",
  upload_failed: "Fallo al subir resultados",
  capture_incompatible: "HLAE incompatible con CS2",
  steam_unavailable: "Steam no está abierto",
  disk_full: "Disco lleno",
  tools_missing: "Faltan herramientas de captura",
  canceled: "Cancelado",
};

const REASON_LABELS: Record<string, string> = {
  capture_incompatible: "HLAE incompatible con CS2",
  steam_unavailable: "Steam no está abierto",
  disk_full: "Disco lleno",
  tools_missing: "Faltan herramientas de captura",
  cs2_running: "CS2 ya está abierto",
  consecutive_failures: "3 fallos seguidos",
};

export function reasonLabel(code: string): string {
  return REASON_LABELS[code] ?? code;
}

export interface PauseReason {
  label: string;
  detail: string | null;
  // The job whose failure paused the worker, when the portal named it.
  job: { id: string; title: string } | null;
}

interface NamedJob {
  job: { id: string; title: string } | null;
  rest: string;
}

// The portal writes an automatic cause as: job <id> "<title>": <raw cause>.
function namedJob(detail: string): NamedJob {
  const match = /^job (\S+) "([^"]*)":\s*([\s\S]*)$/.exec(detail);
  if (match === null) return { job: null, rest: detail };
  const [, id = "", title = "", rest = ""] = match;
  return { job: { id, title }, rest };
}

// A pause reason is "<code>: <detail>" when automatic, free text when an operator wrote it.
export function splitPauseReason(reason: string | null): PauseReason {
  if (reason === null || reason.trim() === "") return { label: "Sin motivo indicado", detail: null, job: null };
  const separator = reason.indexOf(":");
  const code = separator === -1 ? reason.trim() : reason.slice(0, separator).trim();
  const known = REASON_LABELS[code];
  if (known === undefined) return { label: reason, detail: null, job: null };
  const { job, rest } = namedJob(separator === -1 ? "" : reason.slice(separator + 1).trim());
  return { label: known, detail: rest === "" ? null : rest, job };
}

const EVENT_LABELS: Record<string, string> = {
  created: "Trabajo creado",
  demo_uploaded: "Demo subida",
  queued: "Puesto en cola",
  claimed: "Tomado por el worker",
  stage: "Cambio de fase",
  phase_uploading: "Empieza la subida de resultados",
  done: "Terminado",
  failed: "Fallido",
  requeued: "Devuelto a la cola",
  lease_expired: "El worker dejó de responder",
  cancel_requested: "Cancelación pedida",
  canceled: "Cancelado",
  retried: "Reintentado por un operador",
  boosted: "Prioridad cambiada",
  worker_created: "Worker creado",
  worker_paused: "Worker pausado",
  worker_resumed: "Worker reanudado",
  worker_auto_paused: "Worker pausado automáticamente",
  worker_revoked: "Worker revocado",
  user_access_changed: "Acceso de usuario cambiado",
  user_limits_changed: "Límites de usuario cambiados",
  device_linked: "Dispositivo vinculado",
  device_revoked: "Dispositivo revocado",
};

export function eventLabel(type: string): string {
  return EVENT_LABELS[type] ?? type;
}

function codeLabel(code: string): string {
  const failure = FAILURE_CODES.find((candidate) => candidate === code);
  return failure === undefined ? reasonLabel(code) : FAILURE_LABELS[failure];
}

// Why the portal took a job away from its worker, as lostCause writes it in queue-store.
function leaseText(detail: string): string {
  const limit = /^(running|uploading) past its limit of (\d+) s$/.exec(detail);
  if (limit !== null) {
    const doing = limit[1] === "running" ? "en curso" : "subiendo resultados";
    return `seguía ${doing} pasado su límite de ${formatDuration(Number(limit[2]))}`;
  }
  const match = /^lease expired at (\S+)$/.exec(detail);
  const at = match === null ? Number.NaN : Date.parse(match[1] ?? "");
  return Number.isNaN(at) ? detail : `su reserva caducó a las ${formatClock(at)}`;
}

// "<code>: <raw cause>" with the code translated; free text passes through untouched.
function causeText(detail: string): string {
  const separator = detail.indexOf(": ");
  if (separator === -1) return codeLabel(detail);
  const { job, rest } = namedJob(detail.slice(separator + 2));
  const cause = job === null ? leaseText(rest) : `en "${job.title}": ${rest}`;
  return `${codeLabel(detail.slice(0, separator))}: ${cause}`;
}

function limitsText(detail: string): string {
  const match = /^maxActive (\d+|default), dailySeconds (\d+|default)$/.exec(detail);
  if (match === null) return detail;
  const [, active = "default", daily = "default"] = match;
  const noun = active === "1" ? "activo" : "activos";
  const activeText = active === "default" ? "activos a la vez por defecto" : `${active} ${noun} a la vez`;
  const dailyText =
    daily === "default" ? "tiempo diario por defecto" : `${formatDuration(Number(daily))} de máquina al día`;
  return `${activeText}, ${dailyText}`;
}

function createdText(detail: string): string {
  const match = /^([a-z_]+), (\d+) s$/.exec(detail);
  if (match === null) return detail;
  const kind = JOB_KINDS.find((candidate) => candidate === match[1]);
  if (kind === undefined) return detail;
  return `${KIND_LABELS[kind]}, estimación de ${formatDuration(Number(match[2]))}`;
}

// The portal stores event details as terse English keys; this is what the operator reads.
export function eventDetailText(type: string, detail: string | null): string | null {
  if (detail === null || detail.trim() === "") return null;
  const amount = /^(?:attempt )?(\d+)(?: s| bytes)?$/.exec(detail);
  const value = amount === null ? null : Number(amount[1]);
  if (type === "created") return createdText(detail);
  if (type === "demo_uploaded" && value !== null) return formatBytes(value);
  if (type === "queued" && detail === "demo already stored") return "la demo ya estaba en el portal";
  if (type === "claimed" && value !== null) return `intento ${value}`;
  if (type === "phase_uploading" && value !== null) return `${formatDuration(value)} de máquina`;
  if (type === "boosted") {
    return detail === "front" ? "subido al frente" : detail === "reset" ? "prioridad quitada" : detail;
  }
  if (type === "stage") {
    const stage = STAGES.find((candidate) => candidate === detail);
    return stage === undefined ? detail : STAGE_LABELS[stage];
  }
  if (type === "user_access_changed") {
    const access = CLOUD_ACCESS.find((candidate) => candidate === detail);
    return access === undefined ? detail : ACCESS_LABELS[access];
  }
  if (type === "user_limits_changed") return limitsText(detail);
  if (type === "requeued" || type === "failed" || type === "canceled" || type === "worker_auto_paused") {
    return causeText(detail);
  }
  return detail;
}

export function actorLabel(actor: string): string {
  if (actor === "system") return "Sistema";
  if (actor.startsWith("worker:")) return "Worker";
  if (actor.startsWith("admin:")) return "Operador";
  if (actor.startsWith("user:")) return "Usuario";
  return actor;
}

const ACTION_ERRORS: Record<string, string> = {
  invalid_state: "El estado ya había cambiado. La vista se ha actualizado.",
  demo_gone: "La demo ya no está en el portal, no se puede reintentar.",
  unauthorized: "Tu sesión ha caducado. Vuelve a entrar.",
  forbidden: "No tienes permiso para esta acción.",
  not_found: "Ya no existe.",
  network: "No se pudo conectar con el portal.",
  rate_limited: "Demasiadas peticiones seguidas. Espera un minuto y vuelve a intentarlo.",
  code_confirmation_required: "Ese no es el código que muestra ClipHub Studio. Cópialo tal como aparece allí.",
};

// "http_502" and the like: the portal or its proxy failed, nothing the person did.
function serverFailure(code: string): string | null {
  const match = /^http_(5\d\d)$/.exec(code);
  return match === null ? null : `El portal ha fallado al responder (error ${match[1]}).`;
}

export function actionErrorText(code: string): string {
  const known = ACTION_ERRORS[code];
  if (known !== undefined) return known;
  const failure = serverFailure(code);
  if (failure !== null) return `${failure} No se ha hecho la acción; vuelve a intentarlo.`;
  return `No se pudo completar la acción (${code}).`;
}

export function loadErrorText(code: string): string {
  if (code === "network") return "No se pudo conectar con el portal.";
  if (code === "unauthorized") return "Tu sesión ha caducado. Vuelve a entrar.";
  if (code === "forbidden") return "Tu cuenta no tiene permiso para ver esto.";
  if (code === "rate_limited") return "Demasiadas peticiones seguidas. Espera un minuto y vuelve a intentarlo.";
  if (code === "bad_response") return "El portal devolvió datos que este panel no entiende.";
  return serverFailure(code) ?? `El portal respondió con un error (${code}).`;
}
