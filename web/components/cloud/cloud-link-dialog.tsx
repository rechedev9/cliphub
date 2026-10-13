'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Check, Copy, ExternalLink, RefreshCw } from 'lucide-react';
import { cloudApi, CloudApiError } from '@/lib/api/cloud';
import { publishCloudAccount } from '@/lib/cloud/account-store';
import { cloudLinkPhase, type CloudLinkPhase } from '@/lib/cloud/account-view';
import { writeClipboardText } from '@/lib/clipboard-write';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

export type CloudLinkDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

type StartError = { code?: string; status?: number };

const STARTING: CloudLinkPhase = { kind: 'starting' };

function openInBrowser(url: string): void {
  // Electron hands external https URLs to the system browser and denies the popup.
  window.open(url, '_blank', 'noopener,noreferrer');
}

/** Links this PC to a ClipHub account: shows the code, opens the portal and waits for the confirmation. */
export function CloudLinkDialog({ open, onOpenChange }: CloudLinkDialogProps): ReactNode {
  const account = useCloudAccount();
  const [startError, setStartError] = useState<StartError | null>(null);
  const [starting, setStarting] = useState(false);
  const [copied, setCopied] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  /** The code the browser was already opened for, so a poll never opens it twice. */
  const openedFor = useRef<string | null>(null);

  const start = useCallback(async () => {
    setStarting(true);
    setStartError(null);
    setCopied(false);
    try {
      publishCloudAccount(await cloudApi.startLink());
    } catch (err) {
      setStartError(err instanceof CloudApiError ? { code: err.code, status: err.status } : { code: 'service_unavailable' });
    } finally {
      setStarting(false);
    }
  }, []);

  const linked = account.kind === 'ready' && account.account.linked;
  const pendingCode =
    account.kind === 'ready' && account.account.link?.status === 'pending' ? account.account.link.userCode : null;

  // Opening the dialog starts a link unless one is already waiting for the user.
  const wasOpen = useRef(false);
  useEffect(() => {
    if (open && !wasOpen.current && !linked && pendingCode === null) void start();
    wasOpen.current = open;
  }, [open, linked, pendingCode, start]);

  useEffect(() => {
    if (!open) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [open]);

  const phase = starting ? STARTING : cloudLinkPhase({ account, startError, now });
  const pendingUrl = phase.kind === 'pending' ? phase.verifyUrl : null;
  const pendingUserCode = phase.kind === 'pending' ? phase.userCode : null;

  useEffect(() => {
    if (!open || pendingUrl === null || pendingUserCode === null) return;
    if (openedFor.current === pendingUserCode) return;
    openedFor.current = pendingUserCode;
    openInBrowser(pendingUrl);
  }, [open, pendingUrl, pendingUserCode]);

  let body: ReactNode;
  switch (phase.kind) {
    case 'starting':
      body = (
        <p role="status" className="flex items-center gap-2 text-body-sm text-fg-2">
          <span aria-hidden className="studio-spinner text-primary" />
          Preparando el código…
        </p>
      );
      break;
    case 'pending': {
      const { userCode, verifyUrl } = phase;
      body = (
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
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => {
                void writeClipboardText(userCode)
                  .then(() => setCopied(true))
                  .catch(() => setCopied(false));
              }}
            >
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
      break;
    }
    case 'linked':
      body = (
        <>
          <p role="status" className="flex items-center gap-2 text-body text-success">
            <Check aria-hidden className="size-5" />
            Este PC ya está conectado a {phase.name}.
          </p>
          <div className="flex justify-end">
            <Button type="button" size="sm" onClick={() => onOpenChange(false)}>
              Listo
            </Button>
          </div>
        </>
      );
      break;
    case 'denied':
      body = <Retry message="Has rechazado la conexión desde el navegador. Este PC no se ha conectado." onRetry={() => void start()} />;
      break;
    case 'expired':
      body = <Retry message="El código ha caducado antes de confirmarse." onRetry={() => void start()} />;
      break;
    case 'failed':
      body = <Retry message={phase.message} onRetry={() => void start()} />;
      break;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Conecta tu cuenta de ClipHub</DialogTitle>
          <DialogDescription>
            La cuenta solo hace falta para grabar en la nube. Grabar en este PC sigue funcionando sin ella.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">{body}</div>
      </DialogContent>
    </Dialog>
  );
}

function Retry({ message, onRetry }: { message: string; onRetry: () => void }): ReactNode {
  return (
    <>
      <p role="alert" className="text-body-sm text-warning">
        {message}
      </p>
      <div className="flex justify-end">
        <Button type="button" size="sm" variant="outline" onClick={onRetry}>
          <RefreshCw aria-hidden />
          Pedir otro código
        </Button>
      </div>
    </>
  );
}
