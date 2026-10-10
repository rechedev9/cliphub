import type { CloudJob, CloudJobQueue } from './parse.ts';

/** Everything the UI says about a cloud job, decided here so it can be tested without a browser. */

export type CloudJobTone = 'queue' | 'working' | 'ready' | 'failed' | 'neutral';

export type CloudFailureView = {
  message: string;
  /** Offer "Reintentar en la nube". */
  retryCloud: boolean;
  /** Offer "Grabar en este PC". */
  recordLocal: boolean;
};

export type CloudJobView = {
  tone: CloudJobTone;
  /** The one status line, as design section 9 words it. */
  line: string;
  /** Show a progress bar; `percent` null means indeterminate. */
  bar: boolean;
  percent: number | null;
  /** Still moving: drives the fast poll and the shell transport. */
  active: boolean;
  canCancel: boolean;
  /** What cancelling gives up, to confirm first; null when there is no cancel. */
  cancelWarning: string | null;
  canRemove: boolean;
  /** What removing deletes from this PC, to confirm first; null when it deletes nothing. */
  removeWarning: string | null;
  /** Names of the videos that are on disk and verified. */
  playable: string[];
  failure: CloudFailureView | null;
  /** This PC lost its account while the job was moving: offer "Conectar cuenta". */
  needsLink: boolean;
};

/** The PC a job is shown on: its clock, and whether it still holds a ClipHub account. */
export type CloudDevice = { now: number; linked: boolean };

const ESTIMATE_SPREAD = 1.35;
const ESTIMATE_STEP_MINUTES = 5;

function minutesLabel(minutes: number): string {
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours} h` : `${hours} h ${rest} min`;
}

/**
 * A wait as a range: start to start plus 35 percent, on 5 minute steps.
 * Reads after "empieza en": "menos de 5 min", "unos 20 a 30 min".
 */
export function estimateRange(waitSeconds: number): string {
  const minutes = Math.max(0, waitSeconds) / 60;
  const low = Math.floor(minutes / ESTIMATE_STEP_MINUTES) * ESTIMATE_STEP_MINUTES;
  const high = Math.ceil((minutes * ESTIMATE_SPREAD) / ESTIMATE_STEP_MINUTES) * ESTIMATE_STEP_MINUTES;
  if (low < ESTIMATE_STEP_MINUTES) return 'menos de 5 min';
  if (low === high) return `unos ${minutesLabel(low)}`;
  if (high < 60) return `unos ${low} a ${high} min`;
  return `entre ${minutesLabel(low)} y ${minutesLabel(high)}`;
}

function withPercent(label: string, percent: number | null): string {
  return percent === null ? label : `${label} ${percent} %`;
}

function queueLine(queue: CloudJobQueue | null, now: number): string {
  if (queue === null) return 'EN COLA';
  const position = queue.position;
  if (queue.state === 'paused') {
    return position === null ? 'La nube está en pausa. Tu puesto se mantiene' : `La nube está en pausa. Tu puesto (${position}) se mantiene`;
  }
  if (queue.state === 'offline') {
    return position === null
      ? 'La nube no está conectada ahora. Tu puesto se mantiene'
      : `La nube no está conectada ahora. Tu puesto (${position}) se mantiene`;
  }
  const parts = ['EN COLA'];
  if (position !== null) parts.push(`puesto ${position}`);
  if (queue.estimatedStartAt !== null) {
    parts.push(`empieza en ${estimateRange((queue.estimatedStartAt - now) / 1000)}`);
  }
  return parts.join(' · ');
}

function runningLine(job: CloudJob): string {
  switch (job.stage) {
    case 'downloading':
      return 'PREPARANDO EN LA NUBE';
    case 'parsing':
      return 'ANALIZANDO EN LA NUBE';
    case 'capturing':
      return withPercent('GRABANDO EN LA NUBE', job.percent);
    case 'rendering':
      return withPercent('RENDER EN LA NUBE', job.percent);
    case null:
      return 'EN MARCHA EN LA NUBE';
  }
}

// A job only fails with a machine fault after the cloud gave up on it, so the cloud is not paused for it.
const LOCAL_OR_LATER = 'Graba en este PC o vuelve a intentarlo más tarde.';
const RETRY_OR_LOCAL = 'Vuelve a intentarlo o graba en este PC.';

const FAILURES: Record<string, CloudFailureView> = {
  demo_upload_failed: {
    message: 'No se pudo subir la demo a la nube. Revisa tu conexión y vuelve a intentarlo.',
    retryCloud: true,
    recordLocal: true,
  },
  capture_flake: {
    message: `La grabación en la nube se cortó dos veces. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  interrupted: {
    message: `La máquina de la nube se reinició mientras grababa tu vídeo. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  worker_lost: {
    message: `Perdimos la conexión con la máquina de la nube. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  demo_download_failed: {
    message: `La máquina de la nube no pudo recibir tu demo. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  internal: {
    message: `Algo falló en la nube mientras preparaba tu vídeo. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  demo_incompatible: {
    message: 'Esta demo no es compatible con la versión actual de CS2, así que no se puede grabar.',
    retryCloud: false,
    recordLocal: false,
  },
  unplayable_start: {
    message: 'CS2 no pudo reproducir el inicio de esta demo, así que no se puede grabar.',
    retryCloud: false,
    recordLocal: false,
  },
  target_not_found: {
    message: 'El jugador elegido no aparece en esta demo. Carga la demo de nuevo y elige otro jugador.',
    retryCloud: false,
    recordLocal: false,
  },
  spec_mismatch: {
    message: 'La nube analizó la demo y sus jugadas no coinciden con las de este Studio. Actualiza Studio y vuelve a intentarlo.',
    retryCloud: true,
    recordLocal: true,
  },
  invalid_spec: {
    message: 'La nube no aceptó la configuración de este Short. Actualiza Studio y vuelve a intentarlo.',
    retryCloud: true,
    recordLocal: true,
  },
  render_failed: {
    message: `La grabación salió bien, pero el montaje falló en la nube. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  job_failed: {
    message: `El vídeo falló en la nube. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  timeout: {
    message: 'El vídeo tardó demasiado y la nube lo detuvo. Prueba con menos jugadas o graba en este PC.',
    retryCloud: true,
    recordLocal: true,
  },
  upload_failed: {
    message: `El vídeo se creó, pero no pudo salir de la máquina de la nube. ${RETRY_OR_LOCAL}`,
    retryCloud: true,
    recordLocal: true,
  },
  results_expired: {
    message: 'El vídeo se grabó, pero caducó en la nube antes de descargarse en este PC. Vuelve a crearlo.',
    retryCloud: true,
    recordLocal: true,
  },
  download_failed: {
    message: 'El vídeo se grabó, pero llegó dañado a este PC varias veces. Descárgalo desde la web de ClipHub o vuelve a crearlo.',
    retryCloud: true,
    recordLocal: true,
  },
  limit_uploads: {
    message: 'Ya hay dos demos tuyas subiendo a la nube. Espera a que termine una y vuelve a intentarlo.',
    retryCloud: true,
    recordLocal: true,
  },
  limit_storage: {
    message: 'Tus demos ocupan todo tu espacio en la nube. Espera a que terminen tus vídeos en cola y vuelve a intentarlo, o graba en este PC.',
    retryCloud: true,
    recordLocal: true,
  },
  cloud_storage_full: {
    message: 'La nube se quedó sin espacio antes de recibir tu demo. Inténtalo más tarde o graba en este PC.',
    retryCloud: true,
    recordLocal: true,
  },
  capture_incompatible: {
    message: `La nube intentó grabar este vídeo varias veces y la captura falló siempre. Puede deberse a esta demo. ${LOCAL_OR_LATER}`,
    retryCloud: true,
    recordLocal: true,
  },
  steam_unavailable: {
    message: `La nube intentó grabar este vídeo varias veces y Steam no respondió en nuestra máquina. ${LOCAL_OR_LATER}`,
    retryCloud: true,
    recordLocal: true,
  },
  disk_full: {
    message: `La nube intentó grabar este vídeo varias veces y nuestra máquina no tenía espacio. ${LOCAL_OR_LATER}`,
    retryCloud: true,
    recordLocal: true,
  },
  tools_missing: {
    message: `La nube intentó grabar este vídeo varias veces y a nuestra máquina le faltaba una herramienta de grabación. ${LOCAL_OR_LATER}`,
    retryCloud: true,
    recordLocal: true,
  },
};

/** The sentence and the suggested way out for a failure code. */
export function cloudFailureView(failure: { code: string; message: string }): CloudFailureView {
  const known = Object.hasOwn(FAILURES, failure.code) ? FAILURES[failure.code] : undefined;
  if (known !== undefined) return known;
  // A code newer than this Studio: the worker's own sentence is safe to show.
  const message = failure.message.trim();
  return {
    message: message === '' ? `El vídeo falló en la nube. ${RETRY_OR_LOCAL}` : message,
    retryCloud: true,
    recordLocal: true,
  };
}

function readyNames(job: CloudJob): string[] {
  return job.videos.filter((video) => video.ready).map((video) => video.name);
}

const CANCEL_WARNING = {
  uploading_demo: 'La demo que ya se ha subido se descarta y el vídeo no se creará.',
  queued: 'Perderás tu puesto en la cola. Si lo creas otra vez, empezará desde el final.',
  working: 'La nube ya está trabajando en este vídeo. Si cancelas, se pierde lo que lleva hecho.',
} as const;

/** Removing a finished job deletes the only copy Studio can reach: the cloud drops its own once this PC has it. */
function removeWarning(playable: number): string | null {
  if (playable === 0) return null;
  return playable === 1
    ? 'Se borrará el vídeo de este PC y no se podrá volver a descargar. Guárdalo antes si quieres conservarlo.'
    : `Se borrarán los ${playable} vídeos de este PC y no se podrán volver a descargar. Guárdalos antes si quieres conservarlos.`;
}

const STUCK_DOWNLOAD_REMOVE_WARNING =
  'El vídeo no se guardará en este PC. Mientras siga disponible podrás descargarlo desde la web de ClipHub.';

type MovingStatus = 'uploading_demo' | 'queued' | 'running' | 'uploading';

type MovingJob = { job: CloudJob; status: MovingStatus; now: number };

type MovingLook = Pick<CloudJobView, 'tone' | 'line' | 'bar' | 'percent'> & { cancelWarning: string };

/** How a job the cloud still works on reads while the cloud answers. */
function movingLook({ job, status, now }: MovingJob, stale: boolean): MovingLook {
  switch (status) {
    case 'uploading_demo':
      return job.stalled === 'demo_upload'
        ? { tone: 'working', line: 'SUBIENDO DEMO · la subida se cortó y se reintenta sola', bar: true, percent: null, cancelWarning: CANCEL_WARNING.uploading_demo }
        : { tone: 'working', line: withPercent('SUBIENDO DEMO', job.percent), bar: true, percent: job.percent, cancelWarning: CANCEL_WARNING.uploading_demo };
    case 'queued': {
      // An estimate from before an outage is not repeated as if it still held.
      const queue = stale && job.queue !== null ? { ...job.queue, estimatedStartAt: null } : job.queue;
      return { tone: 'queue', line: queueLine(queue, now), bar: false, percent: null, cancelWarning: CANCEL_WARNING.queued };
    }
    case 'running':
      return { tone: 'working', line: runningLine(job), bar: true, percent: job.percent, cancelWarning: CANCEL_WARNING.working };
    case 'uploading':
      return { tone: 'working', line: withPercent('TERMINANDO EN LA NUBE', job.percent), bar: true, percent: job.percent, cancelWarning: CANCEL_WARNING.working };
  }
}

const BASE = { percent: null, playable: [], failure: null, needsLink: false, cancelWarning: null, removeWarning: null } satisfies Partial<CloudJobView>;
const SETTLED = { ...BASE, bar: false, active: false, canCancel: false, canRemove: true } satisfies Partial<CloudJobView>;

function movingView(moving: MovingJob): CloudJobView {
  const { job } = moving;
  const stale = job.stalled === 'portal_unreachable';
  const look = movingLook(moving, stale);
  const view = { ...BASE, ...look, active: true, canCancel: true, canRemove: false };
  if (job.cancelRequested) return { ...view, tone: 'neutral', line: 'CANCELANDO', percent: null, canCancel: false, cancelWarning: null };
  // The line is only the last one this PC heard, so nothing on the row may look like it moves.
  if (stale) return { ...view, tone: 'neutral', line: `Sin conexión con la nube. Último estado: ${look.line}`, bar: false, percent: null };
  return view;
}

/** Maps one cloud job to its tag tone, status line, progress and available actions. */
export function cloudJobView(job: CloudJob, now: number): CloudJobView {
  switch (job.status) {
    case 'uploading_demo':
    case 'queued':
    case 'running':
    case 'uploading':
      return movingView({ job, status: job.status, now });
    case 'downloading': {
      // The cloud already finished this job: only the download is left, so there is nothing to cancel.
      const view = { ...BASE, tone: 'working', bar: true, active: true, canCancel: false } as const;
      return job.stalled === 'download'
        ? { ...view, line: 'DESCARGANDO · la descarga a este PC falla y se reintenta sola', canRemove: true, removeWarning: STUCK_DOWNLOAD_REMOVE_WARNING }
        : { ...view, line: withPercent('DESCARGANDO', job.percent), percent: job.percent, canRemove: false };
    }
    case 'done': {
      const playable = readyNames(job);
      return {
        ...SETTLED,
        tone: 'ready',
        // The portal dropped the files before this PC fetched them.
        line: playable.length > 0 ? 'LISTO · guardado en este PC' : 'LISTO · el vídeo ya no está disponible',
        playable,
        removeWarning: removeWarning(playable.length),
      };
    }
    case 'failed': {
      const failure = cloudFailureView(job.failure ?? { code: '', message: '' });
      return { ...SETTLED, tone: 'failed', line: failure.message, failure };
    }
    case 'canceled':
      return { ...SETTLED, tone: 'neutral', line: 'CANCELADO' };
    case 'unknown':
      return { ...SETTLED, tone: 'neutral', line: 'Estado desconocido. Actualiza Studio para seguir este vídeo', canRemove: false };
  }
}

const UNLINKED_LINE = 'Este PC no está conectado a tu cuenta. Conéctala para seguir este vídeo';

/** `cloudJobView` on a PC that may have lost its account: without one a moving job cannot be followed or canceled. */
export function cloudJobViewOn(job: CloudJob, device: CloudDevice): CloudJobView {
  const view = cloudJobView(job, device.now);
  if (device.linked || !view.active) return view;
  return {
    ...view,
    tone: 'neutral',
    line: UNLINKED_LINE,
    bar: false,
    percent: null,
    active: false,
    canCancel: false,
    cancelWarning: null,
    canRemove: false,
    removeWarning: null,
    needsLink: true,
  };
}

export function anyCloudJobActive(jobs: readonly CloudJob[], device: CloudDevice): boolean {
  return jobs.some((job) => cloudJobViewOn(job, device).active);
}

/** Jobs that finished since the previous poll, for the "listo" toast. */
export function cloudJobsNewlyReady(prev: readonly CloudJob[], next: readonly CloudJob[]): CloudJob[] {
  const before = new Map(prev.map((job) => [job.id, job.status]));
  return next.filter((job) => {
    const was = before.get(job.id);
    return job.status === 'done' && was !== undefined && was !== 'done';
  });
}

/** Cloud jobs grouped by the local match they came from. */
export function cloudJobsByMatch(jobs: readonly CloudJob[]): Map<string, CloudJob[]> {
  const byMatch = new Map<string, CloudJob[]>();
  for (const job of jobs) {
    const list = byMatch.get(job.localJobId);
    if (list === undefined) byMatch.set(job.localJobId, [job]);
    else list.push(job);
  }
  return byMatch;
}

const SUBMIT_ERRORS: Record<string, string> = {
  limit_active: 'Ya tienes el máximo de vídeos en la nube a la vez. Espera a que termine uno o graba en este PC.',
  limit_daily: 'Este vídeo no cabe en el tiempo de nube que te queda hoy. Elige menos jugadas, vuelve mañana o graba en este PC.',
  limit_full_demo: 'Ya tienes un vídeo largo en la nube. Espera a que termine o graba en este PC.',
  cloud_queue_full: 'La cola de la nube está llena ahora mismo. Inténtalo más tarde o graba en este PC.',
  cloud_storage_full: 'La nube se ha quedado sin espacio por ahora. Inténtalo más tarde o graba en este PC.',
  limit_storage: 'Tus demos ocupan todo tu espacio en la nube. Espera a que terminen tus vídeos en cola o graba en este PC.',
  limit_uploads: 'Ya hay dos demos tuyas subiendo a la nube. Espera a que termine una y vuelve a intentarlo.',
  studio_version_mismatch: 'Tu Studio y la nube usan versiones distintas. Actualiza ClipHub Studio y vuelve a intentarlo.',
  cloud_access_pending: 'Tu cuenta está pendiente de aprobación. Hasta entonces puedes grabar en este PC.',
  cloud_access_blocked: 'Tu cuenta no tiene acceso a la nube. Puedes grabar en este PC.',
  portal_unreachable: 'No se pudo conectar con la nube. Revisa tu conexión o graba en este PC.',
  not_linked: 'Este PC no está conectado a tu cuenta de ClipHub. Conecta tu cuenta y vuelve a intentarlo.',
  unauthorized: 'Este PC ya no está conectado a tu cuenta de ClipHub. Conecta tu cuenta de nuevo.',
  job_not_parsed: 'La demo todavía se está analizando. Espera a que termine y vuelve a intentarlo.',
  unknown_segment: 'Alguna jugada elegida ya no existe en esta partida. Vuelve a elegir las jugadas.',
  demo_too_large: 'La demo supera el tamaño máximo que admite la nube. Grábala en este PC.',
  invalid_spec: 'La nube no aceptó la configuración de este Short. Actualiza Studio y vuelve a intentarlo.',
  kind_unavailable: 'La nube todavía no admite este tipo de vídeo. Grábalo en este PC.',
  service_unavailable: 'El servicio local de ClipHub no responde. Cierra y vuelve a abrir Studio.',
  not_found: 'Esta partida ya no está en este PC. Carga la demo de nuevo para crear el Short.',
  not_configured: 'La nube no está configurada en esta instalación de Studio. Graba en este PC.',
};

/** One sentence per portal or local rejection of a cloud submit. */
export function cloudSubmitErrorMessage(error: { code?: string; status?: number }): string {
  if (error.code !== undefined && Object.hasOwn(SUBMIT_ERRORS, error.code)) return SUBMIT_ERRORS[error.code];
  if (error.status === 401) return SUBMIT_ERRORS.unauthorized;
  if (error.status === 404) return 'Esta versión de Studio no puede grabar en la nube. Actualiza Studio o graba en este PC.';
  return 'La nube rechazó este Short por un motivo que Studio no reconoce. Actualiza Studio o graba en este PC.';
}
