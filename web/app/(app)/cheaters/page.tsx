'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Loader2, RefreshCw, ShieldAlert, UploadCloud } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match } from '@/lib/api/types';
import {
  ANTICHEAT_STATUS,
  fetchAnticheat,
  fetchDossier,
  startAnticheat,
  type AnticheatDocument,
  type AnticheatDossier,
  type AnticheatPlayer,
} from '@/lib/api/anticheat';
import { DossierDialog } from '@/components/cheaters/dossier-dialog';
import { PlayerDetail, PlayerSummaryRow } from '@/components/cheaters/player-detail';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { matchDateLabel } from '@/lib/format';
import { cn } from '@/lib/utils';

/** How often a running analysis is re-polled, in milliseconds. */
const POLL_INTERVAL_MS = 3000;

function DemoPicker({
  matches,
  selected,
  onSelect,
}: {
  matches: Match[];
  selected: string | null;
  onSelect: (jobId: string) => void;
}): ReactNode {
  return (
    <nav aria-label="Demos analizables" className="flex flex-col gap-2">
      {matches.map((match) => (
        <button
          key={match.id}
          type="button"
          onClick={() => onSelect(match.id)}
          aria-current={selected === match.id ? 'true' : undefined}
          className={cn(
            'studio-panel studio-panel-interactive flex flex-col items-start gap-1 rounded-lg px-4 py-3 text-left transition-colors',
            selected === match.id && 'border-primary/50 bg-primary/8',
          )}
        >
          <span className="font-[family-name:var(--font-display)] text-sm font-semibold uppercase tracking-tight text-foreground">
            {match.map}
          </span>
          <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.1em] text-muted-foreground">
            {match.score ? `${match.score} · ` : ''}
            {matchDateLabel(match)}
          </span>
        </button>
      ))}
    </nav>
  );
}

/** The analysis panel for the selected demo, across every lifecycle state. */
function AnalysisPanel({
  document,
  loading,
  error,
  starting,
  onStart,
  expanded,
  onToggle,
  onOpenDossier,
  dossierPendingFor,
  dossierError,
}: {
  document: AnticheatDocument | null;
  loading: boolean;
  error: string | null;
  starting: boolean;
  onStart: () => void;
  expanded: string | null;
  onToggle: (steamId: string) => void;
  onOpenDossier: (player: AnticheatPlayer) => void;
  dossierPendingFor: string | null;
  dossierError: string | null;
}): ReactNode {
  if (loading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-16 w-full rounded-lg" />
        <Skeleton className="h-16 w-full rounded-lg" />
        <Skeleton className="h-16 w-full rounded-lg" />
      </div>
    );
  }

  if (error !== null) {
    return (
      <StudioEmptyState
        icon={ShieldAlert}
        title="No se pudo leer el análisis"
        description={error}
        compact
        actions={
          <Button onClick={onStart} disabled={starting}>
            {starting ? <Loader2 className="animate-spin" aria-hidden /> : <RefreshCw aria-hidden />}
            REINTENTAR
          </Button>
        }
      />
    );
  }

  if (document === null) {
    return (
      <StudioEmptyState
        icon={ShieldAlert}
        title="Esta demo aún no se ha analizado"
        description="El análisis lee la demo una vez y puntúa a los diez jugadores. No abre CS2 ni HLAE, no toca la partida y no cambia nada del reel."
        compact
        actions={
          <Button onClick={onStart} disabled={starting}>
            {starting ? <Loader2 className="animate-spin" aria-hidden /> : <ShieldAlert aria-hidden />}
            ANALIZAR DEMO
          </Button>
        }
      />
    );
  }

  if (document.status === ANTICHEAT_STATUS.running) {
    return (
      <StudioEmptyState
        icon={Loader2}
        title="Analizando la demo"
        description="Se está recorriendo la demo tick a tick para medir puntería, información y tiempos de reacción. Puedes salir de esta pantalla; el análisis sigue. Si lleva demasiado tiempo aquí, reinicia el análisis."
        compact
        actions={
          <Button variant="outline" onClick={onStart} disabled={starting}>
            {starting ? <Loader2 className="animate-spin" aria-hidden /> : <RefreshCw aria-hidden />}
            REINICIAR ANÁLISIS
          </Button>
        }
      />
    );
  }

  if (document.status === ANTICHEAT_STATUS.failed || document.report === undefined) {
    return (
      <StudioEmptyState
        icon={ShieldAlert}
        title="El análisis falló"
        description={document.failure_reason ?? 'La demo no pudo analizarse.'}
        compact
        actions={
          <Button onClick={onStart} disabled={starting}>
            {starting ? <Loader2 className="animate-spin" aria-hidden /> : <RefreshCw aria-hidden />}
            REINTENTAR
          </Button>
        }
      />
    );
  }

  const report = document.report;

  return (
    <div className="flex flex-col gap-6">
      <div className="studio-panel flex flex-col gap-2 rounded-lg px-4 py-3.5">
        <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
          {report.match.map} · {report.match.rounds} rondas · línea base {report.baseline.id}
        </span>
        <p className="text-sm leading-6 text-muted-foreground">{report.baseline.description}</p>
        {report.baseline.measured ? null : (
          <p className="text-sm leading-6 text-foreground/80">
            Esta línea base no está medida sobre un corpus de demos: es una estimación. Las puntuaciones son
            orientativas hasta que la calibres con <code>zv demo anticheat calibrate</code>.
          </p>
        )}
      </div>

      {dossierError === null ? null : (
        <p role="alert" className="text-sm leading-6 text-destructive">
          No se pudo preparar el expediente: {dossierError}
        </p>
      )}

      <ul className="studio-panel divide-y divide-border/60 overflow-hidden rounded-xl">
        {report.players.map((player) => (
          <li key={player.steamid64} className="flex flex-col">
            <PlayerSummaryRow
              player={player}
              expanded={expanded === player.steamid64}
              onToggle={() => onToggle(player.steamid64)}
            />
            {expanded === player.steamid64 ? (
              <PlayerDetail
                player={player}
                onOpenDossier={onOpenDossier}
                dossierPending={dossierPendingFor === player.steamid64}
              />
            ) : null}
          </li>
        ))}
      </ul>

      <section className="flex flex-col gap-2 border-t border-border/60 pt-5">
        <h3 className="font-[family-name:var(--font-mono)] text-xs uppercase tracking-[0.16em] text-muted-foreground">
          Qué es y qué no es esto
        </h3>
        <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm leading-6 text-muted-foreground">
          {report.limitations.map((limitation) => (
            <li key={limitation}>{limitation}</li>
          ))}
        </ul>
      </section>
    </div>
  );
}

export default function CheatersPage(): ReactNode {
  const [matches, setMatches] = useState<Match[] | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [document, setDocument] = useState<AnticheatDocument | null>(null);
  const [loading, setLoading] = useState(false);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [dossier, setDossier] = useState<AnticheatDossier | null>(null);
  const [dossierPendingFor, setDossierPendingFor] = useState<string | null>(null);
  // A dossier that fails to load must not replace the report the user is
  // reading, so it carries its own error instead of the panel-level one.
  const [dossierError, setDossierError] = useState<string | null>(null);

  // The selected job is read inside the poll timer, which must not restart on
  // every render; a ref keeps the timer stable while still seeing the latest id.
  const selectedRef = useRef<string | null>(null);
  selectedRef.current = selected;

  useEffect(() => {
    void (async () => {
      try {
        const rows = await api.listMatches();
        setMatches(rows);
        setSelected((current) => current ?? rows[0]?.id ?? null);
      } catch {
        // A demo list that cannot load is an empty screen, never a crash; the
        // empty state below already routes the user to the upload flow.
        setMatches([]);
      }
    })();
  }, []);

  const load = useCallback(async (jobId: string, background: boolean) => {
    if (!background) setLoading(true);
    try {
      const doc = await fetchAnticheat(jobId);
      // A late response for a demo the user already navigated away from must
      // not overwrite the panel they are looking at now.
      if (selectedRef.current !== jobId) return;
      setDocument(doc);
      setError(null);
    } catch (err) {
      if (selectedRef.current !== jobId) return;
      setError(err instanceof Error ? err.message : 'error desconocido');
    } finally {
      if (!background) setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selected === null) return;
    setDocument(null);
    setExpanded(null);
    setError(null);
    setDossierError(null);
    void load(selected, false);
  }, [selected, load]);

  // Poll only while an analysis is actually running, so a settled screen makes
  // no requests at all.
  useEffect(() => {
    if (selected === null || document?.status !== ANTICHEAT_STATUS.running) return;
    const timer = window.setInterval(() => {
      void load(selected, true);
    }, POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [selected, document?.status, load]);

  const start = useCallback(async () => {
    if (selected === null) return;
    setStarting(true);
    setError(null);
    try {
      await startAnticheat(selected);
      await load(selected, true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error desconocido');
    } finally {
      setStarting(false);
    }
  }, [selected, load]);

  const openDossier = useCallback(
    async (player: AnticheatPlayer) => {
      if (selected === null) return;
      setDossierPendingFor(player.steamid64);
      setDossierError(null);
      try {
        setDossier(await fetchDossier(selected, player.steamid64));
      } catch (err) {
        setDossierError(err instanceof Error ? err.message : 'error desconocido');
      } finally {
        setDossierPendingFor(null);
      }
    },
    [selected],
  );

  const toggle = useCallback((steamId: string) => {
    setExpanded((current) => (current === steamId ? null : steamId));
  }, []);

  let body: ReactNode;
  if (matches === null) {
    body = <Skeleton className="h-64 w-full rounded-xl" />;
  } else if (matches.length === 0) {
    body = (
      <StudioEmptyState
        icon={ShieldAlert}
        title="No hay demos que analizar"
        description="Sube una demo de CS2 y podrás pasarla por CheaterDetect sin abrir el juego."
        compact
        actions={
          <Button asChild className="font-[family-name:var(--font-display)] tracking-[0.06em]">
            <Link href="/upload">
              <UploadCloud aria-hidden />
              SUBIR UNA DEMO
            </Link>
          </Button>
        }
      />
    );
  } else {
    body = (
      <div className="grid gap-8 lg:grid-cols-[minmax(200px,260px)_1fr] lg:gap-10">
        <DemoPicker matches={matches} selected={selected} onSelect={setSelected} />
        <AnalysisPanel
          document={document}
          loading={loading}
          error={error}
          starting={starting}
          onStart={() => void start()}
          expanded={expanded}
          onToggle={toggle}
          onOpenDossier={(player) => void openDossier(player)}
          dossierPendingFor={dossierPendingFor}
          dossierError={dossierError}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8 sm:gap-10">
      <StudioPageHeader
        title="CHEATERDETECT"
        description="Analiza una demo subida y compara la puntería, la información y los tiempos de reacción de cada jugador con una distribución de referencia medida. Es un detector de anomalías para decidir qué revisar a mano, no un veredicto."
      />

      {body}

      <DossierDialog dossier={dossier} onClose={() => setDossier(null)} />
    </div>
  );
}
