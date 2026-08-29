'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  ChevronRight,
  CloudUpload,
  Cog,
  FileVideo,
  Info,
  ListChecks,
  Loader2,
  LockKeyhole,
  Monitor,
  X,
} from 'lucide-react';
import { api } from '@/lib/api';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import type { DemoPlayer, RosterMatch } from '@/lib/api/types';
import { aggregateGroupedSeriesRoster } from '@/lib/api/series-roster';
import { groupSeriesDemos } from '@/lib/series-grouping';
import { prettyMapName } from '@/lib/format';
import { navSection } from '@/lib/nav';
import { seriesTitle } from '@/lib/series-status';
import { Wordmark } from '@/components/brand/wordmark';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';
import { StudioDataRow } from '@/components/studio/data-row';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { PlayerPicker } from '@/components/upload/player-picker';

const NAV = navSection('/upload');
const HOME = navSection('/onboarding');

/** Upload pipeline stage; seriesMode, not the stage, picks the spinner layout. */
type Stage = 'idle' | 'scanning' | 'picking' | 'parsing';

/** One dropped demo's roster-scan state. */
type ScanRow =
  | { fileName: string; status: 'scanning' }
  | { fileName: string; status: 'scanned'; jobId: string; players: DemoPlayer[]; match?: RosterMatch }
  | { fileName: string; status: 'error'; reason?: string };

/** One scanned demo's parse state after the player is picked (series mode). */
type ParseRow = { jobId: string; label: string; status: 'parsing' | 'done' | 'skipped' | 'error' };

/** Empty-roster scan: treat as a bad demo, not a transient failure. */
const ZERO_PLAYERS_HINT = 'Sin jugadores — ¿seguro que es una demo de CS2?';

/** True when an API error means the local analysis service is unreachable. */
function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

/** A scanned demo's short label: prettified map name, else its file name. */
function rowLabel(row: Extract<ScanRow, { status: 'scanned' }>): string {
  return row.match ? prettyMapName(row.match.map) : row.fileName;
}

/** Static dropzone pipeline copy; rail colour is positional. */
const PIPELINE_STEPS = [
  {
    n: '01',
    icon: Cog,
    title: 'ANÁLISIS AUTOMÁTICO',
    copy: 'Parseamos la demo y puntuamos cada ronda: clutches, aces, multi-kills.',
  },
  {
    n: '02',
    icon: ListChecks,
    title: 'ELIGES LAS JUGADAS',
    copy: 'Repasas la lista de jugadas detectadas y marcas las que entran en el reel.',
  },
  {
    n: '03',
    icon: Monitor,
    title: 'RENDER EN TU RIG',
    copy: 'Captura y edición en tu propio PC. 9:16 para Shorts o 16:9 para largo.',
  },
] as const;

/** The pipeline rail recedes step by step, so the row reads left-to-right. */
const STEP_RAIL_CLASS = ['bg-primary', 'bg-primary/55', 'bg-primary/25'] as const;

/** No-login upload: one demo or a bo3/bo5 series, then pick whose POV to clip. */
export default function UploadPage() {
  const router = useRouter();
  const [stage, setStage] = useState<Stage>('idle');
  const [seriesId, setSeriesId] = useState<string | null>(null);

  // Single-demo state (seriesMode === false).
  const [fileName, setFileName] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [players, setPlayers] = useState<DemoPlayer[]>([]);
  const [match, setMatch] = useState<RosterMatch | null>(null);

  // Series state (seriesMode === true).
  const [scanRows, setScanRows] = useState<ScanRow[]>([]);
  const [parseRows, setParseRows] = useState<ParseRow[]>([]);

  const seriesMode = scanRows.length > 0;

  const [error, setError] = useState<string | null>(null);
  const [warning, setWarning] = useState<string | null>(null);

  const reset = useCallback((message: string | null) => {
    setError(message);
    setWarning(null);
    setStage('idle');
    setSeriesId(null);
    setFileName(null);
    setJobId(null);
    setPlayers([]);
    setMatch(null);
    setScanRows([]);
    setParseRows([]);
  }, []);

  // --- Single-demo flow (identical outcome to the pre-series behaviour) ---

  const runScan = useCallback(
    async (file: File) => {
      setError(null);
      setWarning(null);
      setFileName(file.name);
      setStage('scanning');
      try {
        const scan = await api.scanDemo(file);
        if (scan.players.length === 0) {
          reset(
            'El escaneo no encontró jugadores en esa demo. ¿Seguro que es una demo de CS2? Prueba con otro archivo .dem.',
          );
          return;
        }
        setJobId(scan.jobId);
        setPlayers(scan.players);
        setMatch(scan.match ?? null);
        setStage('picking');
      } catch (err) {
        reset(
          isServiceUnavailable(err)
            ? 'El servicio de análisis está offline. Arráncalo y vuelve a intentarlo.'
            : 'No se pudo escanear esa demo. Prueba con otro archivo .dem.',
        );
      }
    },
    [reset],
  );

  const onPickSingle = useCallback(
    async (steamId: string, destination: 'highlights' | 'full-demo' = 'highlights') => {
      if (stage !== 'picking' || seriesMode || !jobId) return;
      setError(null);
      setStage('parsing');
      try {
        const parsed = await api.parseDemo({ jobId, steamId });
        router.push(
          destination === 'full-demo' ? '/full-demo/' + parsed.id : '/matches/' + parsed.id,
        );
      } catch (err) {
        reset(
          isServiceUnavailable(err)
            ? 'El servicio de análisis está offline. Arráncalo y vuelve a intentarlo.'
            : 'No se pudieron extraer los highlights de ese jugador. Elige otro.',
        );
      }
    },
    [stage, seriesMode, jobId, router, reset],
  );

  // --- Series flow (2+ demos dropped) ---

  const runSeriesScan = useCallback(
    async (files: File[]) => {
      const sid = crypto.randomUUID();
      setError(null);
      setWarning(null);
      setSeriesId(sid);
      setScanRows(files.map((f) => ({ fileName: f.name, status: 'scanning' })));
      setStage('scanning');

      let sawOffline = false;
      const settle = files.map((file, i) =>
        api
          .scanDemo(file, { seriesId: sid })
          .then((scan): ScanRow => {
            if (scan.players.length === 0) return { fileName: file.name, status: 'error', reason: ZERO_PLAYERS_HINT };
            const row: ScanRow = { fileName: file.name, status: 'scanned', jobId: scan.jobId, players: scan.players };
            if (scan.match) row.match = scan.match;
            return row;
          })
          .catch((err): ScanRow => {
            // One demo's rejection must never sink the others: swallow it here
            // and surface it as a failed row (and the shared offline flag).
            if (isServiceUnavailable(err)) sawOffline = true;
            return { fileName: file.name, status: 'error' };
          })
          .then((row) => {
            // Land each result as it settles so rows resolve live, not in a batch.
            setScanRows((prev) => {
              const next = [...prev];
              next[i] = row;
              return next;
            });
            return row;
          }),
      );

      const rows = await Promise.all(settle);
      const scanned = rows.filter((r) => r.status === 'scanned');
      const failed = rows.filter((r) => r.status === 'error');

      if (scanned.length === 0) {
        reset(
          sawOffline
            ? 'El servicio de análisis está offline. Arráncalo y vuelve a intentarlo.'
            : 'No se pudo escanear ninguna de las demos. Prueba con otros archivos .dem.',
        );
        return;
      }
      if (failed.length > 0) {
        setWarning(
          `No se pudieron escanear ${failed.length} de ${rows.length} demos: ${failed.map((r) => r.fileName).join(', ')}.`,
        );
      }
      setStage('picking');
    },
    [reset],
  );

  const scannedRows = useMemo(
    () => scanRows.filter((r): r is Extract<ScanRow, { status: 'scanned' }> => r.status === 'scanned'),
    [scanRows],
  );
  const logicalMapGroups = useMemo(() => groupSeriesDemos(scannedRows), [scannedRows]);
  const aggregated = useMemo(() => aggregateGroupedSeriesRoster(scannedRows), [scannedRows]);

  const onPickSeries = useCallback(
    async (steamId: string) => {
      if (stage !== 'picking' || !seriesMode || !seriesId) return;
      setError(null);
      const rows: ParseRow[] = scannedRows.map((r) => {
        const hasPlayer = r.players.some((p) => p.steamId === steamId);
        return { jobId: r.jobId, label: rowLabel(r), status: hasPlayer ? 'parsing' : 'skipped' };
      });
      setParseRows(rows);
      setStage('parsing');

      await Promise.allSettled(
        rows.map(async (row, i) => {
          if (row.status === 'skipped') return;
          const next: ParseRow['status'] = await api
            .parseDemo({ jobId: row.jobId, steamId })
            .then((): ParseRow['status'] => 'done')
            .catch((): ParseRow['status'] => 'error');
          setParseRows((prev) => {
            const copy = [...prev];
            copy[i] = { ...copy[i], status: next };
            return copy;
          });
        }),
      );

      router.push('/series/' + seriesId);
    },
    [stage, seriesMode, seriesId, scannedRows, router],
  );

  const onFiles = useCallback(
    (files: File[]) => {
      if (stage !== 'idle' || files.length === 0) return;
      // A single demo keeps the original single-match experience; 2+ demos run
      // the series flow (and mint the shared series id).
      if (files.length === 1) void runScan(files[0]);
      else void runSeriesScan(files);
    },
    [stage, runScan, runSeriesScan],
  );

  // --- Header copy ---

  // Reachable singular: 2+ demos dropped but only one scan survived.
  const mapCount = logicalMapGroups.length;
  let headerTitle = 'ANALIZA CUALQUIER DEMO';
  let headerDescription: ReactNode = (
    <>Suelta un .dem, un .rar/.zip o varios — una serie bo3/bo5 completa — y forja las mejores jugadas en un reel. Sin login.</>
  );
  if (seriesMode) {
    if (stage === 'scanning') {
      headerTitle = 'ANALIZANDO LA SERIE';
      headerDescription = <>Escaneando {scanRows.length} demos de la serie…</>;
    } else if (stage === 'picking') {
      headerTitle = seriesTitle(mapCount);
      headerDescription = (
        <>Elige un jugador y forjaremos sus mejores jugadas en {scannedRows.map(rowLabel).join(', ')}.</>
      );
    } else if (stage === 'parsing') {
      headerTitle = seriesTitle(mapCount);
      headerDescription =
        mapCount === 1 ? (
          <>Forjando los highlights del jugador en el mapa de la serie…</>
        ) : (
          <>Forjando los highlights del jugador en cada mapa de la serie…</>
        );
    }
  } else if (stage === 'picking') {
    headerTitle = '¿A QUIÉN QUIERES CLIPEAR?';
    headerDescription = <>Elige a un jugador de la demo y forjaremos sus mejores jugadas en un reel.</>;
  }

  // --- Card content ---

  let cardContent: ReactNode;
  if (seriesMode && stage === 'scanning') {
    cardContent = <ScanRowList rows={scanRows} />;
  } else if (seriesMode && stage === 'parsing') {
    cardContent = <ParseRowList rows={parseRows} />;
  } else if (seriesMode && stage === 'picking') {
    cardContent = <PlayerPicker players={aggregated} onPick={onPickSeries} seriesMapCount={mapCount} />;
  } else if (stage === 'scanning' || stage === 'parsing') {
    cardContent = <SingleDemoProgress stage={stage} fileName={fileName} />;
  } else if (stage === 'picking') {
    cardContent = <PlayerPicker players={players} onPick={onPickSingle} match={match ?? undefined} />;
  } else {
    // stage === 'idle': not rendered (the dropzone shows instead), so this
    // branch only exists to keep cardContent exhaustively assigned.
    cardContent = null;
  }

  return (
    <main className="relative min-h-screen">
      {/* Ambient light keeps the standalone upload entry visually connected to
          the Studio shell without competing with the working surface. The washes
          are clipped by their own layer rather than by `overflow-x-hidden` on
          <main>: an overflow value on the page root turns it into a scroll
          container, which silently disables `position: sticky` on the player
          picker's confirm bar. */}
      <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden">
        <div
          className="absolute -top-52 left-1/2 h-[40rem] w-[48rem] -translate-x-1/2"
          style={{
            background:
              'radial-gradient(ellipse at center, color-mix(in oklch, var(--primary) 9%, transparent), transparent 70%)',
          }}
        />
        <div
          className="absolute top-[38rem] -right-40 size-[28rem]"
          style={{
            background:
              'radial-gradient(circle at center, color-mix(in oklch, var(--stream) 3.5%, transparent), transparent 70%)',
          }}
        />
      </div>

      {/*
        /upload lives outside the app group, so it has no shell container to key
        breakpoints to. It declares its own: every step below is measured against
        this column, not against the viewport.
      */}
      <div className="@container/upload relative mx-auto flex min-h-screen w-full max-w-[1536px] flex-col px-4 sm:px-6 lg:px-12">
        <header className="relative flex min-h-[68px] items-center justify-between border-b border-border py-3">
          <Link href={HOME.href} aria-label="Inicio de ClipHub" className="inline-flex min-h-11 items-center">
            <Wordmark />
          </Link>
          <div
            aria-hidden
            className="absolute inset-y-0 left-1/2 hidden -translate-x-1/2 items-center border-b-2 border-primary px-5 font-mono text-meta uppercase tracking-wider text-primary @[46rem]/upload:flex"
          >
            <CloudUpload className="mr-3 size-4" />
            {NAV.number} — {NAV.label}
          </div>
          <Button variant="ghost" size="sm" asChild>
            <Link href={HOME.href}>
              <ArrowLeft className="size-4" />
              Volver
            </Link>
          </Button>
        </header>

        <div className="flex flex-1 flex-col py-8 sm:py-10">
          <StudioPageHeader title={headerTitle} description={headerDescription} />

          <div className="mt-7 sm:mt-8">
            {stage === 'idle' ? (
              <div className="flex flex-col gap-4">
                <DemoDropzone onFiles={onFiles} />
                {error ? (
                  <p
                    role="alert"
                    className="flex items-start gap-2.5 border border-destructive/45 bg-destructive/8 px-4 py-3 text-body-sm text-destructive"
                  >
                    <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
                    {error}
                  </p>
                ) : null}
                <PipelineSteps />
              </div>
            ) : (
              <div className="flex flex-col gap-3">
                {warning ? (
                  <div
                    role="alert"
                    className="flex items-start justify-between gap-3 border border-warning/45 bg-warning/10 py-2 pr-2 pl-4 text-body-sm text-warning"
                  >
                    <span className="min-w-0 self-center">{warning}</span>
                    <button
                      type="button"
                      aria-label="Descartar aviso"
                      onClick={() => setWarning(null)}
                      className="grid size-10 shrink-0 place-items-center text-warning/80 transition-colors duration-(--dur-instant) hover:text-warning focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-warning"
                    >
                      <X className="size-4" />
                    </button>
                  </div>
                ) : null}
                {/* The padding step is the one PlayerPicker's sticky confirm bar
                    bleeds against, so the two are keyed to the same container
                    step rather than to a viewport breakpoint. */}
                <Card className="studio-panel-raised p-4 @[40rem]/upload:p-6">{cardContent}</Card>
              </div>
            )}
          </div>
        </div>

        <footer className="flex min-h-16 flex-wrap items-center justify-between gap-3 border-t border-border py-4 font-mono text-meta uppercase tracking-wider text-fg-3">
          <span className="inline-flex items-center gap-3">
            <LockKeyhole className="size-4 text-primary" />
            <strong className="font-normal text-primary">Procesamiento local y privado</strong>
            {/* A 1px rule, not a `|` glyph: --fg-4 is a hairline colour and may
                never carry text, and a real rule keeps the mono run even. */}
            <span aria-hidden className="hidden h-3 w-px bg-border-strong @[34rem]/upload:inline-block" />
            <span className="hidden @[34rem]/upload:inline">Tus archivos, tu equipo, tu control.</span>
          </span>
          <span className="inline-flex items-center gap-3">
            <Info className="size-4" />
            Formato: .dem / .dem.zst / .rar / .zip
            <span aria-hidden className="h-3 w-px bg-border-strong" />
            Máx. 10 demos
          </span>
        </footer>
      </div>
    </main>
  );
}

/** Three "cómo funciona" panels on a receding rail. */
function PipelineSteps(): ReactNode {
  return (
    <ol aria-label="Cómo funciona" className="mt-2 grid gap-4 @[52rem]/upload:grid-cols-3">
      {PIPELINE_STEPS.map((step, index) => (
        <li key={step.n} className="studio-panel relative flex flex-col gap-4 p-5 sm:p-6">
          <span aria-hidden className={`absolute inset-x-0 top-0 h-0.5 ${STEP_RAIL_CLASS[index]}`} />
          <div className="flex items-center justify-between gap-3">
            <IconTile icon={step.icon} size="md" depth="inset" />
            <span className="font-mono text-title tabular-nums text-fg-3">{step.n}</span>
          </div>
          <div className="flex min-w-0 flex-col gap-2">
            <h2 className="font-display text-body-lg font-bold uppercase text-fg-1">{step.title}</h2>
            <p className="text-body-sm text-fg-2">{step.copy}</p>
          </div>
          {index < PIPELINE_STEPS.length - 1 ? (
            <ChevronRight
              aria-hidden
              className="absolute top-1/2 -right-4 hidden size-4 -translate-y-1/2 text-primary/70 @[52rem]/upload:block"
            />
          ) : null}
        </li>
      ))}
    </ol>
  );
}

/** The single-demo scan/parse moment: one centered, announced status object. */
function SingleDemoProgress({ stage, fileName }: { stage: 'scanning' | 'parsing'; fileName: string | null }): ReactNode {
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex min-h-[16rem] flex-col items-center justify-center gap-5 px-4 py-12 text-center"
    >
      <span className="grid size-14 place-items-center border border-primary/45 bg-surface-0 text-primary shadow-[var(--elev-1),var(--glow-primary-md)]">
        <Loader2 className="size-6 animate-spin" />
      </span>
      <div className="flex flex-col items-center gap-2">
        <p className="font-display text-title font-bold uppercase text-fg-1">
          {stage === 'scanning' ? 'Escaneando el roster…' : 'Forjando highlights…'}
        </p>
        {fileName ? (
          <p className="inline-flex max-w-full items-center gap-2 font-mono text-body-sm text-fg-2">
            <FileVideo aria-hidden className="size-4 shrink-0" />
            <span className="truncate">{fileName}</span>
          </p>
        ) : null}
      </div>
    </div>
  );
}

/** The per-demo roster-scan progress list shown while a series is scanning. */
function ScanRowList({ rows }: { rows: ScanRow[] }) {
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-2">
      <p className="mb-1 font-mono text-meta uppercase tracking-wider text-fg-3">Escaneando {rows.length} demos</p>
      {rows.map((row, i) => (
        <StudioDataRow
          key={`${row.fileName}-${i}`}
          icon={FileVideo}
          active={row.status === 'scanning'}
          label={row.fileName}
          value={row.status === 'scanned' && row.match ? `${row.match.scoreT}-${row.match.scoreCt}` : undefined}
          status={<ScanRowStatus row={row} />}
        />
      ))}
    </div>
  );
}

/** The right-hand state of one scan row: working, scanned (with map), or failed. */
function ScanRowStatus({ row }: { row: ScanRow }): ReactNode {
  if (row.status === 'scanning') {
    return (
      <StatusTag icon={Loader2} className="[&_svg]:animate-spin">
        Escaneando
      </StatusTag>
    );
  }
  if (row.status === 'scanned') {
    return (
      <StatusTag tone="primary" icon={CheckCircle2}>
        {row.match ? prettyMapName(row.match.map) : 'Escaneada'}
      </StatusTag>
    );
  }
  return (
    <StatusTag tone="danger" icon={AlertTriangle} className="max-w-[11rem] @[40rem]/upload:max-w-xs">
      <span className="min-w-0 truncate" title={row.reason}>
        {row.reason ?? 'Error'}
      </span>
    </StatusTag>
  );
}

/** The per-map parse progress list shown after the player is picked (series). */
function ParseRowList({ rows }: { rows: ParseRow[] }) {
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-2">
      <p className="mb-1 font-mono text-meta uppercase tracking-wider text-fg-3">Forjando highlights en cada mapa</p>
      {rows.map((row, i) => (
        <StudioDataRow
          key={`${row.jobId}-${i}`}
          active={row.status === 'parsing'}
          label={row.label}
          status={<ParseRowStatus status={row.status} />}
        />
      ))}
    </div>
  );
}

/** The right-hand state of one parse row. */
function ParseRowStatus({ status }: { status: ParseRow['status'] }): ReactNode {
  switch (status) {
    case 'parsing':
      return (
        <StatusTag icon={Loader2} className="[&_svg]:animate-spin">
          Forjando
        </StatusTag>
      );
    case 'done':
      return (
        <StatusTag tone="success" icon={CheckCircle2}>
          Lista
        </StatusTag>
      );
    case 'skipped':
      return <StatusTag>Sin este jugador</StatusTag>;
    case 'error':
      return (
        <StatusTag tone="danger" icon={AlertTriangle}>
          Error
        </StatusTag>
      );
  }
}
