'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Check, Copy, ExternalLink, RefreshCw } from 'lucide-react';
import { cloudApi, CloudApiError } from '@/lib/api/cloud';
import { publishCloudAccount } from '@/lib/cloud/account-store';
import { cloudLinkPhase, type CloudLinkPhase } from '@/lib/cloud/account-view';
import { writeClipboardText } from '@/lib/clipboard-write';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { Button } from '@/components/ui/button';

type StartError = { code?: string; status?: number };

const STARTING: CloudLinkPhase = { kind: 'starting' };

function openInBrowser(url: string): void {
  // Electron hands external https URLs to the system browser and denies the popup.
  window.open(url, '_blank', 'noopener,noreferrer');
}

export type CloudLinkFlow = {
  phase: CloudLinkPhase;
  /** A code was requested and has not arrived yet. */
  busy: boolean;
  /** Asks the portal for a new code; the browser opens once it arrives. */
  start: () => void;
  copied: boolean;
  copy: (code: string) => void;
};

export type CloudLinkFlowOptions = {
  /** The surface is on screen: it may open the browser and must track the code's expiry. */
  active: boolean;
  /** Also open the browser for a code that was already waiting when the surface appeared. */
  openPending: boolean;
};

/** The device-link steps shared by the cloud page and the link dialog. */
export function useCloudLinkFlow({ active, openPending }: CloudLinkFlowOptions): CloudLinkFlow {
  const account = useCloudAccount();
  const [startError, setStartError] = useState<StartError | null>(null);
  const [starting, setStarting] = useState(false);
  const [copied, setCopied] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  /** The code the browser was already opened for, so a poll never opens it twice. */
  const openedFor = useRef<string | null>(null);
  const requested = useRef(false);

  const start = useCallback(() => {
    requested.current = true;
    setStarting(true);
    setStartError(null);
    setCopied(false);
    void cloudApi
      .startLink()
      .then(publishCloudAccount)
      .catch((err: unknown) => {
        setStartError(err instanceof CloudApiError ? { code: err.code, status: err.status } : { code: 'service_unavailable' });
      })
      .finally(() => setStarting(false));
  }, []);

  const copy = useCallback((code: string) => {
    void writeClipboardText(code)
      .then(() => setCopied(true))
      .catch(() => setCopied(false));
  }, []);

  const waiting = account.kind === 'ready' && account.account.link?.status === 'pending';
  useEffect(() => {
    if (!active || !waiting) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [active, waiting]);

  const phase = starting ? STARTING : cloudLinkPhase({ account, startError, now });
  const pendingUrl = phase.kind === 'pending' ? phase.verifyUrl : null;
  const pendingCode = phase.kind === 'pending' ? phase.userCode : null;

  useEffect(() => {
    if (!active || pendingUrl === null || pendingCode === null) return;
    if (openedFor.current === pendingCode) return;
    if (!openPending && !requested.current) return;
    openedFor.current = pendingCode;
    openInBrowser(pendingUrl);
  }, [active, openPending, pendingUrl, pendingCode]);

  return { phase, busy: starting, start, copied, copy };
}

/** What the user sees between asking for a code and the PC being linked; the linked state belongs to the caller. */
export function CloudLinkProgress({ flow }: { flow: CloudLinkFlow }): ReactNode {
  const { phase, start, copied, copy } = flow;
  switch (phase.kind) {
    case 'starting':
      return (
        <p role="status" className="flex items-center gap-2 text-body-sm text-fg-2">
          <span aria-hidden className="studio-spinner text-primary" />
          Preparando el código…
        </p>
      );
    case 'pending': {
      const { userCode, verifyUrl } = phase;
      return (
        <>
          <p className="text-body-sm text-fg-2">
            {verifyUrl === null
              ? 'Abre la web de ClipHub en tu navegador, inicia sesión y escribe este código:'
              : 'Se ha abierto la web de ClipHub en tu navegador. Inicia sesión y confirma que ves este código:'}
          </p>
          <p
            aria-label={`Código ${userCode}`}
            className="select-all border border-border-accent bg-surface-2 px-4 py-5 text-center font-mono text-display-sm font-bold tracking-widest text-fg-1"
          >
            {userCode}
          </p>
          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" variant="outline" onClick={() => copy(userCode)}>
              {copied ? <Check aria-hidden /> : <Copy aria-hidden />}
              {copied ? 'Código copiado' : 'Copiar código'}
            </Button>
            {verifyUrl === null ? null : (
              <Button type="button" size="sm" variant="outline-primary" onClick={() => openInBrowser(verifyUrl)}>
                <ExternalLink aria-hidden />
                Abrir el navegador otra vez
              </Button>
            )}
          </div>
          <p role="status" className="flex items-center gap-2 text-body-sm text-fg-3">
            <span aria-hidden className="studio-spinner text-primary" />
            Esperando tu confirmación. El código caduca a los 10 minutos.
          </p>
        </>
      );
    }
    case 'denied':
      return <Retry message="Has rechazado la conexión desde el navegador. Este PC no se ha conectado." onRetry={start} />;
    case 'expired':
      return <Retry message="El código ha caducado antes de confirmarse." onRetry={start} />;
    case 'failed':
      return <Retry message={phase.message} label="Volver a probar" onRetry={start} />;
    case 'linked':
      return null;
  }
}

function Retry({ message, label = 'Pedir otro código', onRetry }: { message: string; label?: string; onRetry: () => void }): ReactNode {
  return (
    <>
      <p role="alert" className="text-body-sm text-warning">
        {message}
      </p>
      <div className="flex justify-start">
        <Button type="button" size="sm" variant="outline" onClick={onRetry}>
          <RefreshCw aria-hidden />
          {label}
        </Button>
      </div>
    </>
  );
}
