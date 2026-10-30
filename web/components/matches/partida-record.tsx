'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Loader2, PlugZap, RefreshCw, Swords, TriangleAlert } from 'lucide-react';
import { api } from '@/lib/api';
import {
  analyzeLibraryDemo,
  getLibraryDemo,
  getLibraryJobDemo,
  getLibraryMatch,
  isLibraryNotFound,
  isLibraryOffline,
  linkLibraryJob,
  type LibraryDemo,
  type LibraryMatch,
} from '@/lib/api/library';
import { MATCHES_HREF, matchHref } from '@/lib/clips/routes';
import { PARSE_STATE_LABEL, demoErrorLabel } from '@/lib/library/partidas';
import { startPollLoop } from '@/lib/poll-loop';
import { EmptyState } from '@/components/records/empty-state';
import { LibraryMatchRecord } from '@/components/matches/library-match-record';
import { MatchRecord } from '@/components/matches/match-record';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
import { Button } from '@/components/ui/button';

type Resolved =
  | { kind: 'loading' }
  | { kind: 'library'; match: LibraryMatch; demo: LibraryDemo }
  | { kind: 'pending'; demo: LibraryDemo; jobId: string | null }
  | { kind: 'legacy' }
  | { kind: 'missing' }
  | { kind: 'offline' };

const PENDING_POLL_MS = 3_000;
const PENDING_IDLE_MS = 15_000;

/**
 * A partida link names either a library match or, from Crear and older
 * links, a job. A job resolves to the library match of its demo, joining the
 * library first when it is not there yet; a job whose demo is gone keeps the
 * Crear record it always had.
 */
async function resolvePartida(id: string): Promise<Resolved> {
  try {
    const match = await getLibraryMatch(id);
    return { kind: 'library', match, demo: await getLibraryDemo(match.demo_id) };
  } catch (error) {
    if (isLibraryOffline(error)) return { kind: 'offline' };
    if (!isLibraryNotFound(error)) throw error;
  }
  const linked = await getLibraryJobDemo(id);
  if (linked !== null) return { kind: 'pending', demo: linked, jobId: id };
  if ((await api.getMatch(id).catch(() => null)) === null) return { kind: 'missing' };
  try {
    return { kind: 'pending', demo: await linkLibraryJob(id), jobId: id };
  } catch {
    return { kind: 'legacy' };
  }
}

/** The /partidas/[id] page: resolves the id, then shows the library record or its scan in progress. */
export function PartidaRecord({ id }: { id: string }): ReactNode {
  const router = useRouter();
  const [resolved, setResolved] = useState<Resolved>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setResolved({ kind: 'loading' });
    resolvePartida(id)
      .then((next) => {
        if (active) setResolved(next);
      })
      .catch(() => {
        if (active) setResolved({ kind: 'missing' });
      });
    return () => {
      active = false;
    };
  }, [id, attempt]);

  const pendingDemoId = resolved.kind === 'pending' ? resolved.demo.id : null;
  const pendingJobId = resolved.kind === 'pending' ? resolved.jobId : null;
  const pendingMatchId = resolved.kind === 'pending' ? resolved.demo.match_id : undefined;

  // A demo with its match is one redirect away; one still scanning is polled until it has it.
  useEffect(() => {
    if (pendingMatchId !== undefined) router.replace(matchHref(pendingMatchId));
  }, [pendingMatchId, router]);

  useEffect(() => {
    if (pendingDemoId === null || pendingMatchId !== undefined) return;
    const stop = startPollLoop({
      tick: async () => {
        const demo = await getLibraryDemo(pendingDemoId);
        setResolved({ kind: 'pending', demo, jobId: pendingJobId });
        return demo.state === 'queued' || demo.state === 'parsing' ? 'fast' : 'idle';
      },
      fastMs: PENDING_POLL_MS,
      idleMs: PENDING_IDLE_MS,
    });
    return stop;
  }, [pendingDemoId, pendingJobId, pendingMatchId]);

  switch (resolved.kind) {
    case 'loading':
      return <TacticalWorkspaceSkeleton />;
    case 'library':
      return <LibraryMatchRecord key={resolved.match.id} match={resolved.match} demo={resolved.demo} />;
    case 'legacy':
      return <MatchRecord matchId={id} />;
    case 'offline':
      return (
        <EmptyState
          icon={PlugZap}
          title="Servicio local sin conexión"
          description="No se pudo leer la partida. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
          actions={
            <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
              Reintentar
            </Button>
          }
        />
      );
    case 'missing':
      return (
        <EmptyState
          icon={Swords}
          title="Partida no encontrada"
          description="Puede que se haya quitado de la biblioteca o que el enlace sea antiguo."
          actions={
            <Button asChild variant="outline" size="sm">
              <Link href={MATCHES_HREF}>Ver partidas</Link>
            </Button>
          }
        />
      );
    case 'pending':
      return (
        <PendingDemo
          demo={resolved.demo}
          onRetry={async () => setResolved({ kind: 'pending', demo: await analyzeLibraryDemo(resolved.demo.id), jobId: resolved.jobId })}
          onShowCrear={resolved.jobId === null ? undefined : () => setResolved({ kind: 'legacy' })}
        />
      );
  }
}

function PendingDemo({
  demo,
  onRetry,
  onShowCrear,
}: {
  demo: LibraryDemo;
  onRetry: () => Promise<void>;
  onShowCrear?: () => void;
}): ReactNode {
  const [busy, setBusy] = useState(false);
  if (demo.match_id !== undefined) return <TacticalWorkspaceSkeleton />;
  if (demo.state === 'failed') {
    return (
      <EmptyState
        icon={TriangleAlert}
        title="No se pudo analizar la demo"
        description={
          <>
            <span className="font-mono text-meta break-all">{demo.file_name}</span>
            <span className="mt-1 block">{demoErrorLabel(demo.error_code)}</span>
          </>
        }
        actions={
          <>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                void onRetry().finally(() => setBusy(false));
              }}
            >
              <RefreshCw aria-hidden />
              Reintentar
            </Button>
            {onShowCrear ? (
              <Button type="button" variant="ghost" size="sm" onClick={onShowCrear}>
                Ver la partida en Crear
              </Button>
            ) : null}
          </>
        }
      />
    );
  }
  return (
    <EmptyState
      icon={Loader2}
      title="Analizando la demo"
      description={
        <>
          <span className="font-mono text-meta break-all">{demo.file_name}</span>
          <span className="mt-1 block">
            {PARSE_STATE_LABEL[demo.state]}. La partida se abre sola cuando el análisis termina: equipos, jugadores, rondas y táctica.
          </span>
        </>
      }
      actions={
        onShowCrear ? (
          <Button type="button" variant="ghost" size="sm" onClick={onShowCrear}>
            Ver la partida en Crear mientras tanto
          </Button>
        ) : undefined
      }
    />
  );
}
