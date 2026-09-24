import { escapeHtml } from './escaping.ts';

// The fatal-error screen is a data: URL with no script, so it never depends on
// the servers that just failed. Its two actions are sentinel links that
// will-navigate intercepts in main; neither URL is ever resolved.
export const RETRY_URL = 'https://retry.cliphub.invalid/';
export const SEND_DIAGNOSTIC_URL = 'https://send-diagnostic.cliphub.invalid/';

/** unavailable: no collector in this build, so there is nothing to send to. */
export type DiagnosticSendState = 'unavailable' | 'idle' | 'sending' | 'sent' | 'queued' | 'failed';

export interface ErrorScreenInput {
  error: unknown;
  title?: string;
  hint?: string;
  logFile: string;
  /** Already-filtered text is not required: the screen is local. It is escaped here. */
  logTail: string;
  send: DiagnosticSendState;
  /** True when diagnostics were already enabled before this report. */
  consented: boolean;
  supportCode: string;
}

const DEFAULT_TITLE = 'ClipHub Studio no pudo arrancar';
const DEFAULT_HINT =
  'Si un antivirus ha bloqueado o puesto en cuarentena archivos de ClipHub, restáuralos y vuelve a abrir la app.';

export function errorScreenHtml(input: ErrorScreenInput): string {
  return `<!doctype html><html lang="es"><head><meta charset="utf-8">
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'">
    <style>
      body{font:16px system-ui;background:#0a0a0a;color:#eee;padding:2rem}
      .actions{display:flex;flex-wrap:wrap;align-items:center;gap:.75rem;margin-top:1rem}
      a.retry,a.send,span.send{display:inline-block;padding:.6rem 1.2rem;font-weight:600;text-decoration:none;border-radius:4px}
      a.retry{background:#22d9ee;color:#04121a}
      a.send{border:1px solid #22d9ee;color:#22d9ee}
      span.send{border:1px solid #444;color:#999}
      .note{max-width:44rem;margin:.75rem 0 0;color:#999;font-size:14px;line-height:1.5}
      .status{margin:.75rem 0 0;font-size:14px;color:#ccc}
      .status.ok{color:#4ade80}
      .status.error{color:#f87171}
    </style></head>
    <body>
      <h2>${escapeHtml(input.title || DEFAULT_TITLE)}</h2>
      <p>${escapeHtml(input.error)}</p>
      <p style="color:#999">${escapeHtml(input.hint || DEFAULT_HINT)} Registro completo: ${escapeHtml(input.logFile)}</p>
      <div class="actions">
        <a class="retry" href="${RETRY_URL}">Reintentar</a>
        ${sendAction(input.send)}
      </div>
      ${sendDetail(input)}
      <pre style="background:#111;padding:1rem;overflow:auto;max-height:40vh;font-size:12px">${escapeHtml(input.logTail)}</pre>
    </body></html>`;
}

function sendAction(state: DiagnosticSendState): string {
  if (state === 'idle' || state === 'failed') return `<a class="send" href="${SEND_DIAGNOSTIC_URL}">Enviar este diagnóstico</a>`;
  if (state === 'sending') return '<span class="send" aria-busy="true">Enviando…</span>';
  return '';
}

function sendDetail(input: ErrorScreenInput): string {
  const code = escapeHtml(input.supportCode);
  switch (input.send) {
    case 'idle':
      return input.consented
        ? '<p class="note">Los diagnósticos ya están activados: el botón envía ahora lo pendiente en lugar de esperar al siguiente envío automático.</p>'
        : '<p class="note">Envía este error, el final del registro y un resumen del equipo (versión de Windows, GPU, CPU, memoria y espacio libre). '
          + 'Se ocultan rutas, credenciales y correos, y se borra a los 30 días. Al enviarlo se activan los diagnósticos; puedes desactivarlos en Ajustes.</p>';
    case 'sent':
      return `<p class="status ok" role="status">Enviado · código ${code}</p>`;
    case 'queued':
      return `<p class="status" role="status">No se pudo enviar ahora; se enviará en cuanto haya conexión · código ${code}</p>`;
    case 'failed':
      return '<p class="status error" role="alert">No se pudo guardar tu elección; no se ha enviado nada.</p>';
    default:
      return '';
  }
}
