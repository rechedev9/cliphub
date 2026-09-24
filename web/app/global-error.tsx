'use client';

import { useEffect, useState, type CSSProperties, type ReactElement } from 'react';
import { useStudioTelemetry } from '@/hooks/use-studio-telemetry';
import { writeClipboardText } from '@/lib/clipboard-write';
import { recordRendererError } from '@/lib/desktop-telemetry';
import { diagnosticLines, diagnosticText } from '@/lib/diagnostic-summary';

/**
 * The last boundary: a crash in the root layout itself, where `app/(app)/
 * error.tsx` never gets a chance to render. It replaces the root layout, which
 * means the design tokens in globals.css are not guaranteed to be present — a
 * stylesheet that failed to load is one of the things that lands a user here.
 *
 * So this file is the one place in `web/` that deliberately hard-codes its
 * colours instead of reading tokens: a recovery screen that depends on the
 * thing that broke is not a recovery screen. The values mirror --surface-1,
 * --surface-2, --fg-1, --fg-2 and --primary.
 */
const PAGE: CSSProperties = {
  minHeight: '100svh',
  margin: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '24px',
  backgroundColor: '#0e1220',
  color: '#eef3f8',
  fontFamily: 'system-ui, sans-serif',
};

const PANEL: CSSProperties = {
  maxWidth: '34rem',
  width: '100%',
  padding: '28px',
  borderRadius: '10px',
  border: '1px solid #3a4560',
  backgroundColor: '#171d2e',
};

const EYEBROW: CSSProperties = {
  margin: 0,
  fontSize: '12px',
  letterSpacing: '0.2em',
  textTransform: 'uppercase',
  color: '#22d9ee',
};

const TITLE: CSSProperties = { margin: '12px 0 0', fontSize: '24px', lineHeight: 1.15 };
const BODY: CSSProperties = { margin: '12px 0 0', fontSize: '15px', lineHeight: 1.6, color: '#aab7ca' };
const DIAGNOSTIC: CSSProperties = {
  margin: '20px 0 0',
  padding: '12px 14px',
  borderRadius: '6px',
  border: '1px solid #3a4560',
  backgroundColor: '#0b0f1a',
  display: 'grid',
  gridTemplateColumns: 'auto minmax(0, 1fr)',
  columnGap: '16px',
  rowGap: '6px',
  fontSize: '13px',
  lineHeight: 1.45,
};
const DIAGNOSTIC_LABEL: CSSProperties = { margin: 0, color: '#8593a8' };
const DIAGNOSTIC_VALUE: CSSProperties = { margin: 0, color: '#eef3f8', wordBreak: 'break-all', fontVariantNumeric: 'tabular-nums' };
const COPY_LABEL = { idle: 'Copiar diagnóstico', copied: 'Diagnóstico copiado', failed: 'No se pudo copiar' } as const;
const ACTIONS: CSSProperties ={ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '12px', marginTop: '20px' };
const SECONDARY: CSSProperties = {
  minHeight: '44px',
  padding: '0 18px',
  borderRadius: '6px',
  border: '1px solid #3a4560',
  backgroundColor: 'transparent',
  color: '#eef3f8',
  fontSize: '14px',
  fontWeight: 600,
  cursor: 'pointer',
};
const ACTION: CSSProperties = {
  minHeight: '44px',
  padding: '0 20px',
  borderRadius: '6px',
  border: 0,
  backgroundColor: '#22d9ee',
  color: '#08131c',
  fontSize: '14px',
  fontWeight: 700,
  cursor: 'pointer',
};

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}): ReactElement {
  const status = useStudioTelemetry();
  const [route, setRoute] = useState<string | undefined>(undefined);
  const [copy, setCopy] = useState<keyof typeof COPY_LABEL>('idle');

  useEffect(() => {
    console.error('[cliphub] global error', error);
    recordRendererError('global.error', error, { digest: error.digest });
    // The root layout is gone, so read the route straight from the window.
    setRoute(window.location.pathname);
  }, [error]);

  const lines = diagnosticLines({
    supportCode: status?.supportCode,
    sessionId: status?.sessionId,
    digest: error.digest,
    route,
    message: error.message,
  });
  const copyDiagnostic = (): void => {
    void writeClipboardText(diagnosticText(lines))
      .then(() => setCopy('copied'))
      .catch(() => setCopy('failed'));
  };

  return (
    <html lang="es">
      <body style={PAGE}>
        <main role="alert" style={PANEL}>
          <p style={EYEBROW}>ClipHub Studio</p>
          <h1 style={TITLE}>Studio no ha podido arrancar</h1>
          <p style={BODY}>
            Ha fallado la aplicación entera, no solo una pantalla. Tus demos, capturas y renders están en el
            orquestador local y no se han tocado. Si vuelve a ocurrir, revisa <code>studio.log</code>.
          </p>
          <dl style={DIAGNOSTIC}>
            {lines.map((line) => (
              <div key={line.label} style={{ display: 'contents' }}>
                <dt style={DIAGNOSTIC_LABEL}>{line.label}</dt>
                <dd style={DIAGNOSTIC_VALUE}>{line.value}</dd>
              </div>
            ))}
          </dl>
          <div style={ACTIONS}>
            <button type="button" style={ACTION} onClick={reset}>
              Reiniciar Studio
            </button>
            <button type="button" style={SECONDARY} onClick={copyDiagnostic} aria-live="polite">
              {COPY_LABEL[copy]}
            </button>
          </div>
        </main>
      </body>
    </html>
  );
}
