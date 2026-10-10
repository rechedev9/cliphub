import type { CaptureStatus } from '../api/types.ts';
import { PRODUCE_FORMAT, produceHref } from '../clips/routes.ts';
import type { CloudAccount, CloudAccountState } from './parse.ts';
import { estimateRange } from './view.ts';

/** Where a capture runs: this PC, or the ClipHub cloud machine. */
export const CAPTURE_TARGET = { local: 'local', cloud: 'cloud' } as const;
export type CaptureTarget = (typeof CAPTURE_TARGET)[keyof typeof CAPTURE_TARGET];

export const CAPTURE_TARGET_STORAGE_KEY = 'cliphub.capture-target.v1';

/** `?destino=` preselects the target when the hub sends the user back to the producer. */
export const CAPTURE_TARGET_QUERY = 'destino';
const CAPTURE_TARGET_PARAM = { local: 'pc', cloud: 'nube' } as const satisfies Record<CaptureTarget, string>;

export function captureTargetParam(value: string | string[] | null | undefined): CaptureTarget | null {
  if (value === CAPTURE_TARGET_PARAM.local) return CAPTURE_TARGET.local;
  return value === CAPTURE_TARGET_PARAM.cloud ? CAPTURE_TARGET.cloud : null;
}

/** The Short producer of a match with one target preselected. */
export function produceShortHref(matchId: string, target: CaptureTarget): string {
  return `${produceHref(matchId, PRODUCE_FORMAT.short)}&${CAPTURE_TARGET_QUERY}=${CAPTURE_TARGET_PARAM[target]}`;
}

export const CAPTURE_TARGET_LABEL = { local: 'Este PC', cloud: 'Nube ClipHub' } as const satisfies Record<CaptureTarget, string>;

export type CaptureTargetTone = 'neutral' | 'ok' | 'warning' | 'danger';

export type CaptureTargetView = {
  target: CaptureTarget;
  /** False when this Studio has no cloud client: the option stays visible but cannot be picked. */
  cloudSelectable: boolean;
  /** The create button may fire for the selected target. */
  canSubmit: boolean;
  status: { text: string; tone: CaptureTargetTone };
  /** Show "Conectar cuenta" next to the status line. */
  offerLink: boolean;
};

export type ResolveCaptureTargetInput = {
  /** What the user clicked on this screen; wins over everything else. */
  picked?: CaptureTarget | null;
  /** The last choice, from storage. */
  remembered: CaptureTarget | null;
  /** Capture readiness of this PC; null while it is being checked. */
  localStatus: CaptureStatus | null;
  /** CS2 is recording another video on this PC right now. */
  localBusy?: boolean;
  account: CloudAccountState;
  /** Portal job kind this producer creates. */
  kind?: string;
};

const KIND_PLURAL: Record<string, string> = { short: 'Shorts', full_demo: 'vídeos largos' };

function isCaptureTarget(value: unknown): value is CaptureTarget {
  return value === CAPTURE_TARGET.local || value === CAPTURE_TARGET.cloud;
}

/** Reads the remembered target; storage may be unavailable or hold anything. */
export function loadCaptureTarget(storage: Pick<Storage, 'getItem'> | null): CaptureTarget | null {
  try {
    const value = storage?.getItem(CAPTURE_TARGET_STORAGE_KEY);
    return isCaptureTarget(value) ? value : null;
  } catch {
    return null;
  }
}

export function saveCaptureTarget(storage: Pick<Storage, 'setItem'> | null, target: CaptureTarget): void {
  try {
    storage?.setItem(CAPTURE_TARGET_STORAGE_KEY, target);
  } catch {
    // Quota or privacy mode: the choice still holds for this screen.
  }
}

function localCanRecord(status: CaptureStatus | null): boolean {
  return status === 'ready' || status === 'warning';
}

function localStatusLine(input: ResolveCaptureTargetInput, cloudSelectable: boolean): CaptureTargetView['status'] {
  switch (input.localStatus) {
    case null:
      return { text: 'Comprobando este PC…', tone: 'neutral' };
    case 'ready':
      return input.localBusy === true
        ? { text: 'CS2 ocupado: entrará en cola en este PC', tone: 'neutral' }
        : { text: 'CS2 + HLAE listos en este PC', tone: 'ok' };
    case 'warning':
      return { text: 'Revisa las rutas de CS2 y HLAE en Ajustes antes de grabar', tone: 'warning' };
    case 'unconfigured':
      return {
        text: cloudSelectable
          ? 'A este PC le falta CS2 o HLAE. Revísalo en Ajustes o graba en la nube'
          : 'A este PC le falta CS2 o HLAE. Revísalo en Ajustes',
        tone: 'warning',
      };
    case 'offline':
      return { text: 'El servicio local de ClipHub no responde', tone: 'danger' };
  }
}

function minutesLeft(account: CloudAccount): number | null {
  if (account.limits === null || account.usage === null) return null;
  const seconds = account.limits.dailySeconds - account.usage.secondsLast24h - account.usage.secondsCommitted;
  return Math.max(0, Math.floor(seconds / 60));
}

function readyLine(account: CloudAccount, kind: string, left: number | null): string {
  const queue = account.queue;
  const parts: string[] = [];
  if (queue !== null) {
    parts.push(queue.queued === 0 ? 'Sin cola' : `${queue.queued} en cola`);
    const wait = Object.hasOwn(queue.waitSeconds, kind) ? queue.waitSeconds[kind] : null;
    if (wait !== null && wait !== undefined) parts.push(`empieza en ${estimateRange(wait)}`);
  }
  if (left !== null) parts.push(`te quedan ${left} min hoy`);
  return parts.length > 0 ? parts.join(' · ') : 'La nube está lista';
}

type CloudLine = Pick<CaptureTargetView, 'canSubmit' | 'status' | 'offerLink'>;

function blocked(text: string, tone: CaptureTargetTone, offerLink = false): CloudLine {
  return { canSubmit: false, status: { text, tone }, offerLink };
}

function cloudLine(account: CloudAccount, kind: string): CloudLine {
  if (!account.linked) {
    if (account.error === 'portal_unreachable') {
      return blocked('No se puede conectar con la nube ahora. Puedes grabar en este PC', 'danger');
    }
    if (account.error === 'unauthorized') {
      return blocked('Este PC ya no está conectado a tu cuenta. Conéctala de nuevo', 'warning', true);
    }
    if (account.link?.status === 'pending') {
      return blocked(`Confirma el código ${account.link.userCode} en el navegador para terminar`, 'neutral', true);
    }
    return blocked('Conecta tu cuenta para grabar en la nube', 'neutral', true);
  }
  if (account.error !== null) {
    return blocked('No se puede conectar con la nube ahora. Puedes grabar en este PC', 'danger');
  }
  if (account.access === 'pending') {
    return blocked('Tu cuenta está pendiente de aprobación. Hasta entonces puedes grabar en este PC', 'warning');
  }
  if (account.access === 'blocked') return blocked('Tu cuenta no tiene acceso a la nube', 'danger');
  if (account.access === null) return blocked('No se pudo comprobar el acceso de tu cuenta a la nube', 'warning');
  if (!account.kinds.includes(kind)) {
    return blocked(`La nube todavía no admite ${KIND_PLURAL[kind] ?? 'este tipo de vídeo'}`, 'warning');
  }
  if (account.limits !== null && account.usage !== null && account.limits.maxActive > 0 && account.usage.active >= account.limits.maxActive) {
    const max = account.limits.maxActive;
    return blocked(`Ya tienes ${max} ${max === 1 ? 'vídeo' : 'vídeos'} en la nube. Espera a que termine uno`, 'warning');
  }
  const left = minutesLeft(account);
  if (left !== null && left <= 0) {
    return blocked('Has usado tu tiempo de nube de hoy. Vuelve mañana o graba en este PC', 'warning');
  }
  if (account.queue?.state === 'paused') {
    return { canSubmit: true, status: { text: 'La nube está en pausa. Puedes ponerte en cola o grabar en este PC', tone: 'warning' }, offerLink: false };
  }
  if (account.queue?.state === 'offline') {
    return {
      canSubmit: true,
      status: { text: 'La nube no está conectada ahora. Puedes ponerte en cola o grabar en este PC', tone: 'warning' },
      offerLink: false,
    };
  }
  return { canSubmit: true, status: { text: readyLine(account, kind, left), tone: 'ok' }, offerLink: false };
}

/** Decides the selected target, what each option allows and the one status line under the control. */
export function resolveCaptureTarget(input: ResolveCaptureTargetInput): CaptureTargetView {
  const kind = input.kind ?? 'short';
  const cloudSelectable = input.account.kind !== 'unavailable';

  let target: CaptureTarget;
  if (!cloudSelectable) {
    target = CAPTURE_TARGET.local;
  } else if (input.picked !== undefined && input.picked !== null) {
    target = input.picked;
  } else if (input.localStatus === 'unconfigured') {
    // A PC without CS2 or HLAE cannot record: start on the option that can.
    target = CAPTURE_TARGET.cloud;
  } else {
    target = input.remembered ?? CAPTURE_TARGET.local;
  }

  if (target === CAPTURE_TARGET.local) {
    // Local creation is never gated here: it behaves exactly as before this control existed.
    return { target, cloudSelectable, canSubmit: true, status: localStatusLine(input, cloudSelectable), offerLink: false };
  }
  switch (input.account.kind) {
    case 'loading':
      return { target, cloudSelectable, ...blocked('Comprobando la nube…', 'neutral') };
    case 'offline':
      return { target, cloudSelectable, ...blocked('El servicio local de ClipHub no responde', 'danger') };
    case 'unavailable':
      return { target, cloudSelectable, ...blocked('Esta instalación no puede grabar en la nube', 'neutral') };
    case 'ready':
      return { target, cloudSelectable, ...cloudLine(input.account.account, kind) };
  }
}

/** True when the sidebar may add "Puedes grabar en la nube" under a PC that cannot record. */
export function cloudCoversLocalGap(localStatus: CaptureStatus | null, account: CloudAccountState): boolean {
  if (localStatus === null || localCanRecord(localStatus) || localStatus === 'offline') return false;
  return account.kind === 'ready' && account.account.linked && account.account.error === null && account.account.access === 'allowed';
}
