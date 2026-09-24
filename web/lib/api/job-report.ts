// "Reportar un problema con este vídeo": the user flags a finished or failed
// render with one category. The orchestrator records it as a user.report trace
// on the job, so the alert carries the job's own attempt history.

export const JOB_REPORT_CATEGORIES = [
  { value: 'black_video', label: 'Imagen negra o sin partida', hint: 'Se ve el HUD o la mira, pero no el juego.' },
  { value: 'wrong_overlay', label: 'Rótulos o marcador incorrectos', hint: 'Intro, outro, nombres o resultado que no corresponden.' },
  { value: 'audio', label: 'Audio', hint: 'Sin sonido, cortes, volumen o música que no encajan.' },
  { value: 'cuts', label: 'Cortes o jugadas', hint: 'Faltan o sobran jugadas, o los cortes caen mal.' },
  { value: 'other', label: 'Otro problema', hint: 'Cualquier otra cosa que no cuadre en el vídeo.' },
] as const;

export type JobReportCategory = (typeof JOB_REPORT_CATEGORIES)[number]['value'];

export const REPORT_VIDEO_LABEL = 'Reportar un problema con este vídeo';

const CATEGORY_VALUES: ReadonlySet<string> = new Set(JOB_REPORT_CATEGORIES.map((category) => category.value));

export function isJobReportCategory(value: unknown): value is JobReportCategory {
  return typeof value === 'string' && CATEGORY_VALUES.has(value);
}

const JOB_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** First candidate that is an orchestrator job id; null hides the report action. */
export function reportableJobId(...candidates: ReadonlyArray<string | undefined>): string | null {
  return candidates.find((candidate): candidate is string => candidate !== undefined && JOB_ID.test(candidate)) ?? null;
}

export function jobReportUrl(jobId: string): string {
  return `/api/demos/${encodeURIComponent(jobId)}/report`;
}

export type JobReportResult = { ok: true } | { ok: false; status: number; message: string };

type ReportFetch = (url: string, init: { method: 'POST'; headers: Record<string, string>; body: string }) => Promise<{ status: number }>;

/** Posts one report; every failure comes back as Spanish copy for the dialog. */
export async function reportJob(
  jobId: string,
  category: JobReportCategory,
  fetchImpl: ReportFetch = (url, init) => fetch(url, init),
): Promise<JobReportResult> {
  let status: number;
  try {
    ({ status } = await fetchImpl(jobReportUrl(jobId), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ category }),
    }));
  } catch {
    return { ok: false, status: 0, message: REPORT_ERROR.unreachable };
  }
  if (status === 202 || status === 200) return { ok: true };
  return { ok: false, status, message: reportErrorMessage(status) };
}

const REPORT_ERROR = {
  unreachable: 'Studio no responde ahora mismo. Inténtalo de nuevo en unos segundos.',
  rateLimited: 'Ya enviaste un informe de este vídeo hace menos de un minuto.',
  gone: 'Este trabajo ya no existe en Studio.',
  invalid: 'Elige qué ha fallado antes de enviar.',
  generic: 'No se pudo enviar el informe. Inténtalo de nuevo.',
} as const;

function reportErrorMessage(status: number): string {
  if (status === 429) return REPORT_ERROR.rateLimited;
  if (status === 404) return REPORT_ERROR.gone;
  if (status === 400) return REPORT_ERROR.invalid;
  if (status === 503) return REPORT_ERROR.unreachable;
  return REPORT_ERROR.generic;
}

export type ReportDelivery = {
  /** sent: diagnostics are on, so the report leaves the machine with the next upload. */
  tone: 'sent' | 'local';
  text: string;
};

type DeliveryStatus = { available: boolean; enabled: boolean; noticeAcknowledged: boolean; supportCode: string };

/**
 * What the confirmation may promise. With diagnostics off the report stays in
 * the local trace, and the dialog must say so instead of claiming it was sent.
 */
export function reportDelivery(status: DeliveryStatus | null): ReportDelivery {
  if (status === null) return { tone: 'local', text: 'Guardado en Studio.' };
  if (status.available && status.enabled && status.noticeAcknowledged) {
    return { tone: 'sent', text: `Enviado. Código de soporte: ${status.supportCode}` };
  }
  return {
    tone: 'local',
    text: `Guardado solo en este equipo: los diagnósticos están desactivados. Actívalos en Ajustes para que llegue a ClipHub. Código de soporte: ${status.supportCode}`,
  };
}

/** Before sending: warn when the report cannot leave the machine. */
export function reportsStayLocal(status: DeliveryStatus | null): boolean {
  return status !== null && !(status.available && status.enabled && status.noticeAcknowledged);
}
