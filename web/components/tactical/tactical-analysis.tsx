'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { TriangleAlert } from 'lucide-react';
import {
  fetchTacticalDocument,
  fetchTacticalPositions,
  fetchTacticalTendencies,
  isServiceUnavailableError,
} from '@/lib/api/tactical';
import type {
  TacticalDocument,
  TacticalFilter,
  TacticalFrame,
  TacticalTendencies,
} from '@/lib/api/tactical';
import {
  decodePositionsHeader,
  decodeRoundFrames,
  TacticalDecodeError,
  tacticalDecodeErrorMessage,
} from '@/lib/tactical-decode';
import type { PositionsScale } from '@/lib/tactical-decode';
import { filterTacticalRounds, tacticalFilterFromQuery, tacticalFilterToQuery } from '@/lib/tactical-filter';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { TacticalDemoSummary } from '@/components/tactical/tactical-demo-summary';
import { TacticalFilterBar } from '@/components/tactical/tactical-filter-bar';
import { TacticalReplay } from '@/components/tactical/tactical-replay';
import { TacticalRoundList } from '@/components/tactical/tactical-round-list';
import { TacticalTendenciesPanel } from '@/components/tactical/tactical-tendencies';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';

/** The document and the position bytes it describes, loaded once per demo. */
type LoadedAnalysis = { doc: TacticalDocument; blob: ArrayBuffer; scale: PositionsScale };

/**
 * Verifies the blob against the digest the document recorded. A mismatch means
 * the index and the bytes came from different scans, and half a replay is worse
 * than none, so the workspace refuses to draw it. `crypto.subtle` exists in
 * every secure context Studio runs in; where it does not, the check is skipped
 * rather than failing a good analysis.
 */
async function positionsAreStale(doc: TacticalDocument, blob: ArrayBuffer): Promise<boolean> {
  const expected = doc.positions.sha256;
  if (!expected || typeof crypto === 'undefined' || crypto.subtle === undefined) return false;
  const digest = await crypto.subtle.digest('SHA-256', blob);
  const actual = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
  return actual !== expected.toLowerCase();
}

function errorMessage(error: unknown): string {
  if (isServiceUnavailableError(error)) return 'El servicio de análisis local no está disponible.';
  if (error instanceof TacticalDecodeError) return tacticalDecodeErrorMessage(error);
  if (error instanceof UserFacingTacticalError) return error.message;
  return 'No se pudo cargar el análisis táctico. Revisa el estado local y vuelve a intentarlo.';
}

class UserFacingTacticalError extends Error {}

/**
 * The ready workspace: the round index, the 2D replay and the tendencies of one
 * analysed demo.
 *
 * The position blob is fetched once, in parallel with the document, and every
 * round is decoded out of those same bytes through its recorded offset — so
 * changing round is a seek, never another request.
 */
export function TacticalAnalysis({
  jobId,
  generatedAt,
}: {
  jobId: string;
  generatedAt: string;
}): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [loaded, setLoaded] = useState<LoadedAnalysis | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedRound, setSelectedRound] = useState<number | null>(null);
  const [tendencies, setTendencies] = useState<TacticalTendencies | null>(null);
  const [tendenciesError, setTendenciesError] = useState<string | null>(null);
  // Frames decoded so far, keyed by round: switching back to a round already
  // watched costs nothing.
  const frameCache = useRef(new Map<number, TacticalFrame[]>());

  const filter = useMemo<TacticalFilter>(() => tacticalFilterFromQuery(searchParams), [searchParams]);
  const filterQuery = tacticalFilterToQuery(filter);

  useEffect(() => {
    let active = true;
    setLoadError(null);

    void (async () => {
      try {
        // Independent requests: the index and its sidecar always travel together.
        const [doc, blob] = await Promise.all([
          fetchTacticalDocument(jobId),
          fetchTacticalPositions(jobId),
        ]);
        if (await positionsAreStale(doc, blob)) {
          throw new UserFacingTacticalError(
            'Las posiciones no coinciden con el índice (SHA-256 distinto): el análisis está desfasado y hay que repetirlo.',
          );
        }
        if (!active) return;
        frameCache.current = new Map();
        setLoaded({ doc, blob, scale: decodePositionsHeader(blob) });
      } catch (error) {
        if (active) setLoadError(errorMessage(error));
      }
    })();

    return () => {
      active = false;
    };
  }, [jobId]);

  useEffect(() => {
    let active = true;
    setTendenciesError(null);

    void (async () => {
      try {
        const next = await fetchTacticalTendencies(jobId, tacticalFilterFromQuery(new URLSearchParams(filterQuery)));
        if (active) setTendencies(next);
      } catch (error) {
        if (!active) return;
        setTendencies(null);
        setTendenciesError(errorMessage(error));
      }
    })();

    return () => {
      active = false;
    };
  }, [jobId, filterQuery]);

  const applyFilter = useCallback(
    (next: TacticalFilter) => {
      const query = tacticalFilterToQuery(next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router],
  );

  const rounds = useMemo(
    () => (loaded === null ? [] : filterTacticalRounds(loaded.doc.teams, filter, loaded.doc.rounds)),
    [loaded, filter],
  );

  // Keep the selection inside the visible set: a filter that hides the open
  // round moves to the first one it kept instead of leaving an orphan replay.
  useEffect(() => {
    if (rounds.length === 0) return;
    setSelectedRound((current) =>
      current !== null && rounds.some((round) => round.number === current) ? current : rounds[0].number,
    );
  }, [rounds]);

  const round = useMemo(
    () => rounds.find((candidate) => candidate.number === selectedRound) ?? rounds[0],
    [rounds, selectedRound],
  );

  const frames = useMemo<TacticalFrame[]>(() => {
    if (loaded === null || round === undefined) return [];
    const cached = frameCache.current.get(round.number);
    if (cached !== undefined) return cached;
    const offset = loaded.doc.positions.round_offsets.find((entry) => entry.round === round.number);
    if (offset === undefined) return [];
    try {
      const decoded = decodeRoundFrames(loaded.blob, offset, loaded.scale);
      frameCache.current.set(round.number, decoded);
      return decoded;
    } catch {
      // A truncated or mis-offset round must not take the workspace down; the
      // replay says it has no positions and the rest of the analysis stands.
      return [];
    }
  }, [loaded, round]);

  if (loadError !== null) {
    return (
      <StudioEmptyState
        icon={TriangleAlert}
        title="No se pudo abrir el análisis"
        description={
          <span className="font-[family-name:var(--font-mono)] text-sm break-words text-destructive">
            {loadError}
          </span>
        }
        compact
      />
    );
  }

  if (loaded === null) return <TacticalWorkspaceSkeleton />;

  return (
    <div className="flex flex-col gap-6 sm:gap-8">
      <TacticalDemoSummary doc={loaded.doc} generatedAt={generatedAt} />

      <TacticalFilterBar
        doc={loaded.doc}
        filter={filter}
        onChange={applyFilter}
        matched={rounds.length}
        total={loaded.doc.rounds.length}
      />

      <div className="grid gap-6 xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)] xl:items-start">
        <TacticalRoundList
          doc={loaded.doc}
          rounds={rounds}
          filter={filter}
          selected={round?.number ?? null}
          onSelect={setSelectedRound}
        />
        <TacticalReplay doc={loaded.doc} round={round} frames={frames} />
      </div>

      <TacticalTendenciesPanel
        tendencies={tendencies}
        error={tendenciesError}
        roundCount={rounds.length}
      />
    </div>
  );
}
