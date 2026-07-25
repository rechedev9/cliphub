'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, PlugZap, Radar, RefreshCw, TriangleAlert } from 'lucide-react';
import {
  TACTICAL_DEFAULT_SAMPLE_HZ,
  TACTICAL_STATES,
  fetchTacticalStatus,
  isServiceUnavailableError,
  startTacticalAnalysis,
} from '@/lib/api/tactical';
import type { TacticalStatus } from '@/lib/api/tactical';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { TacticalAnalysis } from '@/components/tactical/tactical-analysis';
import { TacticalStateBadge } from '@/components/tactical/tactical-state-badge';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
import { Button } from '@/components/ui/button';
import { startPollLoop } from '@/lib/poll-loop';
import { browserWindowActivity } from '@/lib/window-activity';
import { stateLabel } from '@/lib/tactical-labels';

// The scan is a few seconds of work on a queue, so poll it briskly while it runs
// and let the loop idle once nothing is in flight.
const FAST_POLL_MS = 1200;
const IDLE_POLL_MS = 15000;

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'fallo desconocido';
}

/**
 * The tactical analysis of one demo, from "never scanned" to the workspace.
 *
 * Every lifecycle state the status endpoint can report has a screen here. The
 * `none` state deliberately has a button and no automatic POST: the scan re-parses
 * the whole demo, so it is always an explicit decision.
 */
export function TacticalWorkspace({ jobId }: { jobId: string }): ReactNode {
  const [status, setStatus] = useState<TacticalStatus | null>(null);
  const [offline, setOffline] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  // Bumped to restart the poll loop immediately after a start or a manual retry,
  // instead of waiting out the idle beat the previous state settled into.
  const [pollToken, setPollToken] = useState(0);

  const state = status?.state;
  const settled = state === TACTICAL_STATES.ready || state === TACTICAL_STATES.failed;

  // The poll loop suspends itself while the window is unfocused, which is the
  // right cadence for a refresh and the wrong one for a first paint: a workspace
  // opened on a second monitor would sit on its skeleton until it was clicked.
  // So the first read always happens, and the loop only maintains it.
  useEffect(() => {
    if (browserWindowActivity.isActive()) return;
    let active = true;
    void fetchTacticalStatus(jobId)
      .then((next) => {
        if (active) setStatus(next);
      })
      .catch((error: unknown) => {
        if (active) setOffline(isServiceUnavailableError(error));
      });
    return () => {
      active = false;
    };
  }, [jobId, pollToken]);

  useEffect(() => {
    if (settled) return;
    let active = true;

    const stop = startPollLoop({
      tick: async () => {
        try {
          const next = await fetchTacticalStatus(jobId);
          if (!active) return 'idle';
          setStatus(next);
          setOffline(false);
          return next.state === TACTICAL_STATES.queued || next.state === TACTICAL_STATES.running
            ? 'fast'
            : 'idle';
        } catch (error) {
          // A transient status failure keeps the loop alive at the idle cadence;
          // only a reachability failure changes what the screen says.
          if (active) setOffline(isServiceUnavailableError(error));
          return 'idle';
        }
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });

    return () => {
      active = false;
      stop();
    };
  }, [jobId, settled, pollToken]);

  const start = useCallback(async () => {
    setStarting(true);
    setStartError(null);
    try {
      setStatus(await startTacticalAnalysis(jobId));
      setOffline(false);
      setPollToken((token) => token + 1);
    } catch (error) {
      setStartError(
        isServiceUnavailableError(error)
          ? 'El servicio de análisis local no está disponible.'
          : errorMessage(error),
      );
    } finally {
      setStarting(false);
    }
  }, [jobId]);

  const retryStatus = useCallback(() => {
    setOffline(false);
    setPollToken((token) => token + 1);
  }, []);

  return (
    <div className="flex flex-col gap-8 sm:gap-10">
      <StudioPageHeader
        title="ANÁLISIS TÁCTICO"
        description="Clasificación determinista de rondas, repetición 2D y tendencias. Todo sale de la demo; nada se infiere del vídeo."
        actions={
          <div className="flex items-center gap-3">
            {status ? <TacticalStateBadge state={status.state} /> : null}
            <Button asChild variant="outline" className="font-[family-name:var(--font-mono)] text-xs tracking-[0.14em]">
              <Link href="/tactical">
                <ArrowLeft aria-hidden />
                OTRA DEMO
              </Link>
            </Button>
          </div>
        }
      />

      <TacticalBody
        jobId={jobId}
        status={status}
        offline={offline}
        starting={starting}
        startError={startError}
        onStart={start}
        onRetry={retryStatus}
      />
    </div>
  );
}

function TacticalBody({
  jobId,
  status,
  offline,
  starting,
  startError,
  onStart,
  onRetry,
}: {
  jobId: string;
  status: TacticalStatus | null;
  offline: boolean;
  starting: boolean;
  startError: string | null;
  onStart: () => void;
  onRetry: () => void;
}): ReactNode {
  if (status === null) {
    if (offline) {
      return (
        <StudioEmptyState
          icon={PlugZap}
          title="Servicio local no disponible"
          description="No se pudo contactar con el servicio de análisis local. Arráncalo y vuelve a intentarlo."
          compact
          actions={
            <Button onClick={onRetry} className="font-[family-name:var(--font-display)] tracking-[0.06em]">
              <RefreshCw aria-hidden />
              REINTENTAR
            </Button>
          }
        />
      );
    }
    return <TacticalWorkspaceSkeleton />;
  }

  switch (status.state) {
    case TACTICAL_STATES.none:
      return <TacticalStartPanel starting={starting} error={startError} onStart={onStart} />;
    case TACTICAL_STATES.queued:
    case TACTICAL_STATES.running:
      return <TacticalRunningPanel status={status} />;
    case TACTICAL_STATES.failed:
      return (
        <TacticalFailurePanel
          reason={status.error ?? 'El análisis falló sin motivo registrado.'}
          starting={starting}
          error={startError}
          onRetry={onStart}
        />
      );
    case TACTICAL_STATES.ready:
      return <TacticalAnalysis jobId={jobId} generatedAt={status.generated_at} />;
  }
}

function TacticalStartPanel({
  starting,
  error,
  onStart,
}: {
  starting: boolean;
  error: string | null;
  onStart: () => void;
}): ReactNode {
  return (
    <StudioEmptyState
      icon={Radar}
      title="Esta demo aún no está analizada"
      description={
        <>
          El escaneo vuelve a recorrer la demo entera para clasificar cada ronda y muestrear posiciones a{' '}
          {TACTICAL_DEFAULT_SAMPLE_HZ} Hz, así que nunca se lanza solo. Se ejecuta una vez y queda guardado.
        </>
      }
      actions={
        <Button
          onClick={onStart}
          disabled={starting}
          className="font-[family-name:var(--font-display)] tracking-[0.06em]"
        >
          <Radar aria-hidden />
          {starting ? 'ENVIANDO…' : 'ANALIZAR'}
        </Button>
      }
      note={error ? <span className="text-destructive">{error}</span> : 'ANÁLISIS LOCAL · SIN SUBIR NADA'}
    />
  );
}

function TacticalRunningPanel({ status }: { status: TacticalStatus }): ReactNode {
  return (
    <section
      className="studio-panel studio-panel-raised flex flex-col items-center gap-4 rounded-xl px-6 py-14 text-center sm:px-10"
      aria-live="polite"
    >
      <span className="grid size-12 place-items-center rounded-lg border border-primary/30 bg-background/55 text-primary shadow-inner">
        <Radar className="size-5 animate-pulse motion-reduce:animate-none" aria-hidden />
      </span>
      <h2 className="font-[family-name:var(--font-display)] text-xl font-bold uppercase tracking-tight text-foreground">
        {stateLabel(status.state)}
      </h2>
      <p className="max-w-xl text-[15px] leading-6 text-muted-foreground">
        Se está recorriendo la demo para clasificar rondas y muestrear posiciones. Esta página se actualiza sola
        al terminar.
      </p>
    </section>
  );
}

function TacticalFailurePanel({
  reason,
  starting,
  error,
  onRetry,
}: {
  reason: string;
  starting: boolean;
  error: string | null;
  onRetry: () => void;
}): ReactNode {
  return (
    <StudioEmptyState
      icon={TriangleAlert}
      title="El análisis falló"
      description={
        <span className="font-[family-name:var(--font-mono)] text-sm break-words text-destructive">{reason}</span>
      }
      actions={
        <Button
          onClick={onRetry}
          disabled={starting}
          className="font-[family-name:var(--font-display)] tracking-[0.06em]"
        >
          <RefreshCw aria-hidden />
          {starting ? 'ENVIANDO…' : 'REINTENTAR ANÁLISIS'}
        </Button>
      }
      note={error ? <span className="text-destructive">{error}</span> : undefined}
    />
  );
}
