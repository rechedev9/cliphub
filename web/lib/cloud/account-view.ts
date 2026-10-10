import type { CloudAccount, CloudAccountState } from './parse.ts';

/** What the link dialog and the Settings card show, decided without a browser. */

export type CloudLinkPhase =
  | { kind: 'starting' }
  | { kind: 'pending'; userCode: string; verifyUrl: string | null }
  | { kind: 'linked'; name: string }
  | { kind: 'denied' }
  | { kind: 'expired' }
  | { kind: 'failed'; message: string };

export type CloudLinkPhaseInput = {
  account: CloudAccountState;
  /** The rejection of the last `startLink`, if it failed. */
  startError: { code?: string; status?: number } | null;
  now: number;
};

const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]']);

/** Only an https page (or a loopback portal in development) may be opened from a server-provided URL. */
export function safeVerifyUrl(value: string): string | null {
  try {
    const url = new URL(value);
    if (url.username !== '' || url.password !== '') return null;
    if (url.protocol === 'https:') return url.href;
    return url.protocol === 'http:' && LOOPBACK_HOSTS.has(url.hostname) ? url.href : null;
  } catch {
    return null;
  }
}

function startFailure(error: { code?: string; status?: number }): string {
  if (error.status === 429) return 'Demasiados intentos seguidos. Espera unos minutos y vuelve a probar.';
  if (error.code === 'portal_unreachable') return 'No se pudo conectar con la nube. Revisa tu conexión y vuelve a probar.';
  if (error.code === 'service_unavailable') return 'El servicio local de ClipHub no responde. Cierra y vuelve a abrir Studio.';
  if (error.status === 404 || error.code === 'not_configured') return 'Esta instalación de Studio no puede conectarse a la nube.';
  return 'No se pudo iniciar la conexión de la cuenta. Vuelve a probar.';
}

/** The single state the link dialog is in. */
export function cloudLinkPhase(input: CloudLinkPhaseInput): CloudLinkPhase {
  const { account, startError, now } = input;
  if (account.kind === 'ready' && account.account.linked) {
    const user = account.account.user;
    return { kind: 'linked', name: user?.name || user?.email || 'tu cuenta' };
  }
  if (startError !== null) return { kind: 'failed', message: startFailure(startError) };
  if (account.kind === 'unavailable') return { kind: 'failed', message: startFailure({ status: 404 }) };
  if (account.kind === 'offline') return { kind: 'failed', message: startFailure({ code: 'service_unavailable' }) };
  if (account.kind === 'loading') return { kind: 'starting' };
  const link = account.account.link;
  if (link === null) {
    return account.account.error === 'portal_unreachable'
      ? { kind: 'failed', message: startFailure({ code: 'portal_unreachable' }) }
      : { kind: 'starting' };
  }
  if (link.status === 'denied') return { kind: 'denied' };
  if (link.status === 'expired' || (link.expiresAt !== null && link.expiresAt <= now)) return { kind: 'expired' };
  return { kind: 'pending', userCode: link.userCode, verifyUrl: safeVerifyUrl(link.verifyUrl) };
}

export type CloudAccessTag = { text: string; tone: 'success' | 'warning' | 'danger' | 'neutral' };

export function cloudAccessTag(account: CloudAccount): CloudAccessTag {
  if (account.error === 'portal_unreachable') return { text: 'Sin conexión con la nube', tone: 'warning' };
  switch (account.access) {
    case 'allowed':
      return { text: 'Acceso activo', tone: 'success' };
    case 'pending':
      return { text: 'Pendiente de aprobación', tone: 'warning' };
    case 'blocked':
      return { text: 'Sin acceso', tone: 'danger' };
    case null:
      return { text: 'Acceso sin comprobar', tone: 'neutral' };
  }
}

/** False only once the account is known and this PC has none; while unknown, jobs keep their last status. */
export function cloudDeviceLinked(state: CloudAccountState): boolean {
  return state.kind !== 'ready' || state.account.linked;
}

/** What unlinking costs right now, or null when nothing is in flight. */
export function cloudUnlinkWarning(account: CloudAccount): string | null {
  const active = account.usage?.active ?? 0;
  if (active <= 0) return null;
  const subject = active === 1 ? 'Tienes 1 vídeo en marcha en la nube. Seguirá' : `Tienes ${active} vídeos en marcha en la nube. Seguirán`;
  return `${subject} allí, pero este PC no podrá verlo ni descargarlo hasta que conectes la misma cuenta otra vez.`;
}

function minutes(seconds: number): number {
  return Math.round(seconds / 60);
}

/** Usage lines of the Settings card: today's machine time and the videos in flight. */
export function cloudUsageLines(account: CloudAccount): string[] {
  if (account.usage === null || account.limits === null) return [];
  const lines = [
    `Tiempo de nube en las últimas 24 h: ${minutes(account.usage.secondsLast24h)} de ${minutes(account.limits.dailySeconds)} min`,
  ];
  if (account.usage.secondsCommitted > 0) {
    lines.push(`Reservado por tus vídeos en cola: ${minutes(account.usage.secondsCommitted)} min`);
  }
  lines.push(`Vídeos en la nube ahora: ${account.usage.active} de ${account.limits.maxActive}`);
  return lines;
}
