'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Check, Cloud, Link2, Monitor, type LucideIcon } from 'lucide-react';
import { cloudApi } from '@/lib/api/cloud';
import { RESULTS_ORIGIN, resultsHref, SHORTS_HREF } from '@/lib/clips/routes';
import { refreshCloudAccount } from '@/lib/cloud/account-store';
import {
  cloudConnectionTag,
  cloudLinkIdle,
  cloudQueueLine,
  cloudUnlinkWarning,
  cloudUsageLines,
} from '@/lib/cloud/account-view';
import { CAPTURE_TARGET_LABEL } from '@/lib/cloud/capture-target';
import type { CloudAccount } from '@/lib/cloud/parse';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { CloudLinkProgress, useCloudLinkFlow } from '@/components/cloud/cloud-link-flow';
import { IconTile } from '@/components/studio/icon-tile';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StatusTag } from '@/components/studio/status-tag';
import { WorkflowProgress } from '@/components/studio/workflow-progress';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const PAGE_TITLE = 'Nube ClipHub';
const PAGE_DESCRIPTION =
  'Graba tus Shorts en un servidor de ClipHub en lugar de en este PC. Conecta tu cuenta una vez y elige dónde grabar cada vez que crees un Short.';

const LINK_STEPS = ['Pedir código', 'Confirmar en el navegador', 'PC conectado'] as const;

const NOTE_CLASS = 'border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2';
const EYEBROW_CLASS = 'font-mono text-meta uppercase tracking-widest text-fg-3';

const COMPARISON = [
  {
    icon: Monitor,
    title: CAPTURE_TARGET_LABEL.local,
    facts: [
      'CS2 se abre y graba en este ordenador.',
      'No puedes usar el PC hasta que termine la grabación.',
      'No necesita cuenta ni conexión con la nube.',
      'Shorts y vídeos largos.',
    ],
  },
  {
    icon: Cloud,
    title: CAPTURE_TARGET_LABEL.cloud,
    facts: [
      'Graba un servidor de ClipHub: tu PC queda libre.',
      'La demo se sube y el vídeo se descarga aquí al terminar.',
      'Necesita una cuenta de ClipHub y tiene un tiempo diario.',
      'Solo Shorts por ahora.',
    ],
  },
] as const satisfies ReadonlyArray<{ icon: LucideIcon; title: string; facts: readonly string[] }>;

/** The cloud section: connect this PC to a ClipHub account, see the account, and learn what the cloud changes. */
export function CloudPage(): ReactNode {
  const state = useCloudAccount();
  const linked = state.kind === 'ready' && state.account.linked;
  const flow = useCloudLinkFlow({ active: !linked, openPending: false });
  const tag = cloudConnectionTag(state);

  const step = flow.phase.kind === 'pending' ? 1 : 0;

  let body: ReactNode;
  switch (state.kind) {
    case 'loading':
      body = <Skeleton aria-label="Comprobando la cuenta de ClipHub" className="h-24 w-full rounded-none" />;
      break;
    case 'offline':
      body = <p className={NOTE_CLASS}>El servicio local de ClipHub no responde, así que no se puede leer la cuenta. Cierra y vuelve a abrir Studio.</p>;
      break;
    case 'unavailable':
      body = (
        <p className={NOTE_CLASS}>
          La nube no está configurada en esta instalación de Studio. Grabar en este PC funciona igual que siempre.
        </p>
      );
      break;
    case 'ready':
      if (state.account.linked) body = <LinkedAccount account={state.account} />;
      else if (cloudLinkIdle(state) && !flow.busy && flow.phase.kind === 'starting') body = <ConnectIntro unauthorized={state.account.error === 'unauthorized'} onStart={flow.start} />;
      else body = <CloudLinkProgress flow={flow} />;
      break;
  }

  return (
    <div className="measure-list flex flex-col gap-8">
      <StudioPageHeader
        title={PAGE_TITLE}
        description={PAGE_DESCRIPTION}
        actions={tag === null ? null : <StatusTag tone={tag.tone} icon={Cloud} size="md">{tag.text}</StatusTag>}
      />

      <section aria-labelledby="cloud-connection-title" className="studio-panel flex flex-col gap-5 p-4 @[34rem]/content:p-6">
        <div className="flex items-center gap-4">
          <IconTile icon={Link2} size="md" depth="inset" tone={linked ? 'success' : 'primary'} />
          <div className="flex min-w-0 flex-col gap-1">
            <span className={EYEBROW_CLASS}>Conexión de este PC</span>
            <h2 id="cloud-connection-title" className="font-display text-title font-bold uppercase text-fg-1">
              {linked ? 'Este PC está conectado' : 'Conecta este PC a tu cuenta'}
            </h2>
          </div>
        </div>
        {state.kind === 'ready' && !linked ? <WorkflowProgress steps={LINK_STEPS} current={step} /> : null}
        <div className="flex flex-col gap-4">{body}</div>
      </section>

      <section aria-labelledby="cloud-compare-title" className="flex flex-col gap-3">
        <h2 id="cloud-compare-title" className="font-display text-title font-bold uppercase text-fg-1">
          Este PC o la nube
        </h2>
        <p className="text-body-sm text-fg-2">
          Lo eliges al preparar cada Short, en “Dónde se graba”. El vídeo termina siempre en Resultados, con su origen indicado.
        </p>
        <div className="grid gap-3 @[44rem]/content:grid-cols-2">
          {COMPARISON.map(({ icon: Icon, title, facts }) => (
            <div key={title} className="studio-panel flex flex-col gap-3 p-4">
              <h3 className="flex items-center gap-2.5 font-display text-body-lg font-bold uppercase text-fg-1">
                <Icon aria-hidden className="size-5 shrink-0 text-primary" />
                {title}
              </h3>
              <ul className="flex flex-col gap-1.5 text-body-sm text-fg-2">
                {facts.map((fact) => (
                  <li key={fact} className="flex items-start gap-2">
                    <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-fg-3" />
                    {fact}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

/** Before any code exists: what will happen, and the one click that starts it. */
function ConnectIntro({ unauthorized, onStart }: { unauthorized: boolean; onStart: () => void }): ReactNode {
  return (
    <>
      {unauthorized ? (
        <p role="status" className="text-body-sm text-warning">
          Este PC se desvinculó desde la web de ClipHub. Conéctalo de nuevo para grabar en la nube.
        </p>
      ) : null}
      <p className="text-body-sm text-fg-2">
        Studio te dará un código y abrirá la web de ClipHub en tu navegador. Inicia sesión allí y confirma el código: tu
        contraseña nunca se escribe en Studio. Grabar en este PC sigue funcionando sin cuenta.
      </p>
      <div className="flex flex-wrap gap-3">
        <Button type="button" onClick={onStart}>
          <Link2 aria-hidden />
          Conectar cuenta
        </Button>
      </div>
    </>
  );
}

function LinkedAccount({ account }: { account: CloudAccount }): ReactNode {
  const lines = cloudUsageLines(account);
  const queueLine = cloudQueueLine(account);
  const warning = cloudUnlinkWarning(account);
  const [confirming, setConfirming] = useState(false);
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
      {account.error === 'portal_unreachable' ? (
        <p role="status" className="text-body-sm text-warning">
          No se puede conectar con la nube ahora. Revisa tu conexión a internet.
        </p>
      ) : null}
      {lines.length > 0 || queueLine !== null ? (
        <ul className="flex flex-col gap-1 text-body-sm text-fg-2">
          {lines.map((line) => (
            <li key={line} className="tabular-nums">
              {line}
            </li>
          ))}
          {queueLine === null ? null : <li className="tabular-nums">{queueLine}</li>}
        </ul>
      ) : null}
      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <Link href={SHORTS_HREF}>Crear un Short</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href={resultsHref({ origin: RESULTS_ORIGIN.cloud })}>Ver vídeos de la nube</Link>
        </Button>
      </div>
      <div className="flex flex-col gap-3 border-t border-border-subtle pt-4">
        {confirming && warning !== null ? (
          <p role="alert" className="text-body-sm text-warning">
            {warning}
          </p>
        ) : null}
        {error ? (
          <p role="alert" className="text-body-sm text-destructive">
            {error}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-3">
          {confirming && warning !== null ? (
            <>
              <Button type="button" size="sm" variant="outline" loading={unlinking} loadingText="Desvinculando…" onClick={() => void unlink()}>
                Desvincular de todos modos
              </Button>
              <Button type="button" size="sm" variant="ghost" disabled={unlinking} onClick={() => setConfirming(false)}>
                Seguir conectado
              </Button>
            </>
          ) : (
            <Button
              type="button"
              size="sm"
              variant="outline"
              loading={unlinking}
              loadingText="Desvinculando…"
              onClick={warning === null ? () => void unlink() : () => setConfirming(true)}
            >
              Desvincular este PC
            </Button>
          )}
          <span className="text-body-sm text-fg-3">Este PC dejará de grabar en la nube; grabar aquí no cambia.</span>
        </div>
      </div>
    </>
  );
}
