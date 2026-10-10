'use client';

import { useState, type ReactNode } from 'react';
import { Cloud } from 'lucide-react';
import { cloudApi } from '@/lib/api/cloud';
import { refreshCloudAccount } from '@/lib/cloud/account-store';
import { cloudAccessTag, cloudUnlinkWarning, cloudUsageLines } from '@/lib/cloud/account-view';
import type { CloudAccount } from '@/lib/cloud/parse';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { CloudLinkDialog } from '@/components/cloud/cloud-link-dialog';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const NOTE_CLASS = 'border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2';

/** Settings card for the ClipHub account that the cloud capture option uses. */
export function CloudAccountCard(): ReactNode {
  const state = useCloudAccount();
  const [linkOpen, setLinkOpen] = useState(false);
  const [unlinking, setUnlinking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function unlink(): Promise<void> {
    setUnlinking(true);
    setError(null);
    try {
      await cloudApi.unlink();
    } catch {
      setError('No se pudo desvincular este PC. Vuelve a probar.');
    } finally {
      await refreshCloudAccount();
      setUnlinking(false);
    }
  }

  let status: ReactNode;
  let body: ReactNode;
  switch (state.kind) {
    case 'loading':
      status = <Skeleton aria-label="Comprobando la cuenta de ClipHub" className="h-7 w-32 rounded-none" />;
      body = <Skeleton className="h-11 w-full rounded-none" />;
      break;
    case 'offline':
      status = <StatusTag tone="warning">Sin datos</StatusTag>;
      body = <p className={NOTE_CLASS}>El servicio local de ClipHub no responde, así que no se puede leer la cuenta.</p>;
      break;
    case 'unavailable':
      status = <StatusTag tone="neutral">No disponible</StatusTag>;
      body = (
        <p className={NOTE_CLASS}>
          La nube no está configurada en esta instalación de Studio. Grabar en este PC funciona igual que siempre.
        </p>
      );
      break;
    case 'ready':
      if (state.account.linked) {
        const tag = cloudAccessTag(state.account);
        status = <StatusTag tone={tag.tone}>{tag.text}</StatusTag>;
        body = <LinkedBody account={state.account} unlinking={unlinking} onUnlink={() => void unlink()} />;
      } else {
        status = <StatusTag tone="neutral">Sin conectar</StatusTag>;
        body = (
          <>
            {state.account.error === 'portal_unreachable' ? (
              <p role="status" className="text-body-sm text-warning">
                No se puede conectar con la nube ahora. Revisa tu conexión a internet.
              </p>
            ) : null}
            {state.account.error === 'unauthorized' ? (
              <p role="status" className="text-body-sm text-warning">
                Este PC se desvinculó desde la web de ClipHub. Conéctalo de nuevo para grabar en la nube.
              </p>
            ) : null}
            <div className="flex flex-wrap gap-3">
              <Button type="button" onClick={() => setLinkOpen(true)}>
                Conectar cuenta
              </Button>
            </div>
          </>
        );
      }
      break;
  }

  return (
    <section id="cloud" className="studio-panel flex scroll-mt-20 flex-col gap-5 p-4 @[34rem]/content:p-6" aria-labelledby="cloud-account-title">
      <div className="flex items-center gap-4">
        <IconTile icon={Cloud} size="md" depth="inset" />
        <div className="flex min-w-0 flex-col gap-1">
          <SectionEyebrow label="GRABAR SIN USAR ESTE PC" />
          <h2 id="cloud-account-title" className="font-display text-title font-bold uppercase text-fg-1">
            Nube ClipHub
          </h2>
        </div>
        <div className="ml-auto shrink-0">{status}</div>
      </div>
      <p className="text-body-sm text-fg-2">
        Al crear un Short puedes elegir que lo grabe la máquina de ClipHub en lugar de este PC. Tu demo se sube, el vídeo
        se graba en cola y se descarga aquí cuando está listo. Necesita una cuenta de ClipHub; grabar en este PC no.
      </p>
      {body}
      {error ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
      <CloudLinkDialog open={linkOpen} onOpenChange={setLinkOpen} />
    </section>
  );
}

function LinkedBody({
  account,
  unlinking,
  onUnlink,
}: {
  account: CloudAccount;
  unlinking: boolean;
  onUnlink: () => void;
}): ReactNode {
  const lines = cloudUsageLines(account);
  const warning = cloudUnlinkWarning(account);
  const [confirming, setConfirming] = useState(false);
  return (
    <>
      <div className="flex min-w-0 flex-col gap-0.5 border border-border bg-surface-1 px-3 py-3">
        <p className="truncate font-display text-body font-bold text-fg-1">{account.user?.name || 'Cuenta de ClipHub'}</p>
        {account.user?.email ? <p className="truncate font-mono text-meta text-fg-3">{account.user.email}</p> : null}
      </div>
      {account.access === 'pending' ? (
        <p role="status" className="text-body-sm text-warning">
          Tu cuenta está pendiente de aprobación. Hasta entonces puedes grabar en este PC.
        </p>
      ) : null}
      {account.access === 'blocked' ? (
        <p role="status" className="text-body-sm text-destructive">
          Tu cuenta no tiene acceso a la nube. Puedes seguir grabando en este PC.
        </p>
      ) : null}
      {lines.length > 0 ? (
        <ul className="flex flex-col gap-1 text-body-sm text-fg-2">
          {lines.map((line) => (
            <li key={line} className="tabular-nums">
              {line}
            </li>
          ))}
        </ul>
      ) : null}
      {confirming && warning !== null ? (
        <p role="alert" className="text-body-sm text-warning">
          {warning}
        </p>
      ) : null}
      <div className="flex flex-wrap gap-3">
        {confirming && warning !== null ? (
          <>
            <Button type="button" variant="outline" loading={unlinking} loadingText="Desvinculando…" onClick={onUnlink}>
              Desvincular de todos modos
            </Button>
            <Button type="button" variant="ghost" disabled={unlinking} onClick={() => setConfirming(false)}>
              Seguir conectado
            </Button>
          </>
        ) : (
          <Button
            type="button"
            variant="outline"
            loading={unlinking}
            loadingText="Desvinculando…"
            onClick={warning === null ? onUnlink : () => setConfirming(true)}
          >
            Desvincular
          </Button>
        )}
      </div>
    </>
  );
}
