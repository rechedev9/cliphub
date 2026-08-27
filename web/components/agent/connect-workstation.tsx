'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Check, CircleAlert, Download, Loader2, RefreshCw } from 'lucide-react';
import { useCallback, useEffect, useState, type ReactElement } from 'react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { Wordmark } from '@/components/brand/wordmark';
import { Button } from '@/components/ui/button';
import { api } from '@/lib/api';
import { claimAccountDevice, loadAccountSession, type AccountUser } from '@/lib/api/account';
import type { CaptureReadiness } from '@/lib/api/types';
import { useAgentTransport } from './agent-transport';

type AccountState =
  | { readonly status: 'loading' }
  | { readonly status: 'anonymous' }
  | { readonly status: 'authenticated'; readonly user: AccountUser }
  | { readonly status: 'error'; readonly error: string };

export function ConnectWorkstation(): ReactElement {
  const router = useRouter();
  const search = useSearchParams();
  const pairingCode = search.get('pair');
  const { state: transport, reconnect } = useAgentTransport();
  const [account, setAccount] = useState<AccountState>({ status: 'loading' });
  const [claimError, setClaimError] = useState<string | null>(null);
  const [readiness, setReadiness] = useState<CaptureReadiness | null>(null);

  const loadAccount = useCallback(async () => {
    const result = await loadAccountSession();
    if (result.ok) {
      setAccount({ status: 'authenticated', user: result.value });
    } else if (result.code === 'authentication_required') {
      setAccount({ status: 'anonymous' });
    } else {
      setAccount({ status: 'error', error: result.error });
    }
  }, []);

  useEffect(() => {
    void loadAccount();
  }, [loadAccount]);

  useEffect(() => {
    if (account.status !== 'authenticated' || pairingCode === null) return;
    let active = true;
    void claimAccountDevice(pairingCode).then((result) => {
      if (!active) return;
      if (!result.ok && result.code !== 'pairing_not_found') setClaimError(result.error);
      if (result.ok) router.replace('/connect');
    });
    return () => { active = false; };
  }, [account.status, pairingCode, router]);

  useEffect(() => {
    if (transport.status !== 'ready') return;
    let active = true;
    void api.getCaptureReadiness().then((value) => {
      if (active) setReadiness(value);
    }).catch(() => {
      if (active) setReadiness(null);
    });
    return () => { active = false; };
  }, [transport.status]);

  const nextPath = pairingCode === null ? '/connect' : `/connect?pair=${encodeURIComponent(pairingCode)}`;
  const ready = account.status === 'authenticated' && transport.status === 'ready';
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-[54rem] flex-col justify-center gap-8 px-6 py-12">
      <Wordmark />
      <section className="studio-panel studio-panel-raised flex flex-col gap-7 p-7 sm:p-9">
        <div className="flex flex-col gap-3">
          <SectionEyebrow label="ESTE PC" />
          <h1 className="font-display text-display-sm font-bold text-fg-1">Conecta el motor local</h1>
          <p className="max-w-2xl text-body text-fg-2">
            Chrome será la interfaz. HLAE, FFmpeg, las demos y cada render permanecerán en este equipo.
          </p>
        </div>

        <ol className="grid gap-3 md:grid-cols-3">
          <ConnectionStep number="01" title="Cuenta" ready={account.status === 'authenticated'} detail={accountDetail(account)} />
          <ConnectionStep number="02" title="Agente" ready={transport.status === 'ready'} detail={transportDetail(transport)} />
          <ConnectionStep number="03" title="Herramientas" ready={readiness?.status === 'ready'} detail={readinessDetail(readiness)} />
        </ol>

        {claimError === null ? null : (
          <p role="alert" className="flex items-center gap-2 text-body-sm text-destructive">
            <CircleAlert aria-hidden className="size-4" /> {claimError}
          </p>
        )}

        <div className="flex flex-col gap-3 sm:flex-row">
          {account.status === 'anonymous' ? (
            <Button asChild variant="hero" size="lg">
              <Link href={`/login?next=${encodeURIComponent(nextPath)}`}>INICIAR SESIÓN</Link>
            </Button>
          ) : null}
          {transport.status === 'disconnected' || transport.status === 'error' ? (
            <Button asChild variant="hero" size="lg">
              <Link href="/api/installer" prefetch={false}><Download aria-hidden /> INSTALAR CLIPHUB AGENT</Link>
            </Button>
          ) : null}
          {transport.status === 'error' ? (
            <Button type="button" variant="outline" size="lg" onClick={reconnect}>
              <RefreshCw aria-hidden /> VOLVER A COMPROBAR
            </Button>
          ) : null}
          {ready ? (
            <Button type="button" variant="hero" size="lg" onClick={() => router.push('/onboarding')}>
              ABRIR STUDIO
            </Button>
          ) : null}
        </div>
      </section>
    </main>
  );
}

function ConnectionStep({ number, title, ready, detail }: { number: string; title: string; ready: boolean; detail: string }): ReactElement {
  return (
    <li className="flex min-h-32 flex-col justify-between gap-4 rounded-md border border-border bg-surface-3 p-4 shadow-[var(--elev-0)]">
      <div className="flex items-center justify-between gap-3">
        <span className="font-mono text-meta tracking-wider text-primary">{number}</span>
        {ready ? <Check aria-label="Preparado" className="size-4 text-success" /> : <Loader2 aria-label="Pendiente" className="size-4 text-fg-3" />}
      </div>
      <div>
        <h2 className="font-display text-title font-semibold text-fg-1">{title}</h2>
        <p className="mt-1 text-body-sm text-fg-2">{detail}</p>
      </div>
    </li>
  );
}

function accountDetail(state: AccountState): string {
  switch (state.status) {
    case 'loading': return 'Comprobando la sesión…';
    case 'anonymous': return 'Inicia sesión para vincular este equipo.';
    case 'authenticated': return state.user.email;
    case 'error': return state.error;
  }
}

function transportDetail(state: ReturnType<typeof useAgentTransport>['state']): string {
  switch (state.status) {
    case 'local': return 'Studio local activo.';
    case 'connecting': return 'Buscando el servicio en 127.0.0.1…';
    case 'disconnected': return 'Instala o abre ClipHub Agent.';
    case 'ready': return 'Conexión local autenticada.';
    case 'error': return state.error;
  }
}

function readinessDetail(readiness: CaptureReadiness | null): string {
  if (readiness === null) return 'Se comprobarán CS2, HLAE y FFmpeg.';
  if (readiness.status === 'ready') return 'CS2, HLAE y FFmpeg preparados.';
  return readiness.reason ?? 'Hay requisitos pendientes en este PC.';
}
