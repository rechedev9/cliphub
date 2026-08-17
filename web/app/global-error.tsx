'use client';

import { useEffect, type CSSProperties, type ReactElement } from 'react';

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
const CODE: CSSProperties = {
  margin: '20px 0 0',
  padding: '10px 12px',
  borderRadius: '6px',
  backgroundColor: '#0b0f1a',
  color: '#aab7ca',
  fontFamily: 'ui-monospace, monospace',
  fontSize: '12px',
  overflowX: 'auto',
};
const ACTION: CSSProperties = {
  marginTop: '20px',
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
  useEffect(() => {
    console.error('[cliphub] global error', error);
  }, [error]);

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
          <pre style={CODE}>{error.digest === undefined ? error.message : `${error.message}\n${error.digest}`}</pre>
          <button type="button" style={ACTION} onClick={reset}>
            Reiniciar Studio
          </button>
        </main>
      </body>
    </html>
  );
}
