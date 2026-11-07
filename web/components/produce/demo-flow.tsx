'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, CheckCircle2, ChevronDown, FileVideo, Loader2, SearchX, Unplug, X } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { DemoPlayer, RosterMatch } from '@/lib/api/types';
import { formatUploadBytes, uploadProgress, uploadSent, type UploadProgress } from '@/lib/api/upload';
import { aggregateGroupedSeriesRoster } from '@/lib/api/series-roster';
import { getFaceitScoreboard, type FaceitScoreboard } from '@/lib/api/faceit-scoreboard';
import { MATCH_STATUS_SCANNED } from '@/lib/clips/hub';
import {
  FLOW_QUERY,
  flowHref,
  isJobIdParam,
  PRODUCE_FORMAT,
  produceHref,
  seriesHref,
  type ProduceFormat,
} from '@/lib/clips/routes';
import {
  DEMO_EMPTY_ROSTER_HINT,
  DEMO_SCAN_FAIL_HINT,
  DEMO_SERVICE_OFFLINE_HINT,
  demoParseError,
  demoScanError,
  demoSeriesParseError,
  isDemoServiceUnavailable,
} from '@/lib/demo-parse-flow';
import { startPollLoop } from '@/lib/poll-loop';
import { ROSTER_READY_STATUSES, SCAN_PENDING_STATUSES } from '@/lib/api/types';
import { prettyMapName } from '@/lib/format';
import { classifyFullDemoLoadFailure, fullDemoEmptyState, type FullDemoLoadFailure } from '@/lib/full-demo';
import { FLOW_HOME_DESCRIPTION, FLOW_HOME_TITLE, FLOW_TITLE, PRODUCE_MATCH_MISSING } from '@/lib/produce/copy';
import { groupSeriesDemos } from '@/lib/series-grouping';
import { seriesTitle } from '@/lib/series-status';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { DemoSourceHelp } from '@/components/onboarding/demo-source-help';
import { RecentSteamMatches } from '@/components/onboarding/recent-matches';
import { ShareCodeDoor } from '@/components/onboarding/share-code-door';
import { StatusTag } from '@/components/studio/status-tag';
import { LongOperation } from '@/components/studio/long-operation';
import { FlowHomeHero } from '@/components/produce/flow-home-hero';
import { StudioBackLink } from '@/components/studio/back-link';
import { CreateFlowSteps } from '@/components/studio/create-flow-steps';
import { DemoList } from '@/components/produce/demo-list';
import { StudioDataRow } from '@/components/studio/data-row';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { PlayerPicker } from '@/components/upload/player-picker';

/** Pipeline stage; seriesMode, not the stage, picks the layout. */
type Stage = 'idle' | 'scanning' | 'picking' | 'parsing';

/** Why `?job=` could not resume: the job is gone, or its roster could not be read. */
type ResumeFailure = Exclude<FullDemoLoadFailure, null> | 'missing' | null;

type ScanRow =
  | { fileName: string; status: 'scanning' }
  | { fileName: string; status: 'scanned'; jobId: string; players: DemoPlayer[]; match?: RosterMatch }
  | { fileName: string; status: 'error'; reason?: string };

type ParseRow = { jobId: string; label: string; status: 'parsing' | 'done' | 'skipped' | 'error'; reason?: string };

const SOURCE_KIND = { file: 'file', steam: 'steam' } as const;
type SourceKind = (typeof SOURCE_KIND)[keyof typeof SOURCE_KIND];

const IMPORT_POLL_MS = 1500;
const IMPORT_RETRY_MS = 10000;

const ZERO_PLAYERS_HINT = 'Sin jugadores. ¿Seguro que es una demo de CS2?';

const PICK_TITLE = '¿De quién es el vídeo?';
const PICK_DESCRIPTION = {
  short: 'Un jugador, o de 2 a 5 del mismo equipo para mezclar sus jugadas. Después podrás revisar el contenido antes de crearlo.',
  full: 'Elige el jugador desde cuya vista se graba la partida, o de 2 a 5 del mismo equipo para un resumen con sus mejores jugadas.',
} as const;

const SERIES_NOTE = 'En una serie prepararás un vídeo por mapa después del análisis.';

/** "Mirage · 13 : 9 · 22 rondas"; null when the scan has no match line. */
function matchLine(match: RosterMatch | null): string | null {
  if (match === null) return null;
  return `${prettyMapName(match.map)} · ${match.scoreT} : ${match.scoreCt} · ${match.rounds} rondas`;
}

function rowLabel(row: Extract<ScanRow, { status: 'scanned' }>): string {
  return row.match ? prettyMapName(row.match.map) : row.fileName;
}

export type DemoFlowQuery = Record<string, string | string[] | undefined>;

/**
 * Home of one creation flow: cargar demo → escanear → elegir POV → parsear; series bo3/bo5 walk the same page.
 * `?job=` skips the upload and resumes a `scanned` job at the same picker.
 */
export function DemoFlow({ format, query }: { format: ProduceFormat; query: DemoFlowQuery }): ReactNode {
  const router = useRouter();
  const jobParam = query[FLOW_QUERY.job];
  const homeHref = flowHref(format);
  const resuming = jobParam !== undefined;
  const resumeJobId = isJobIdParam(jobParam) ? jobParam : null;

  const [stage, setStage] = useState<Stage>(resuming ? 'scanning' : 'idle');
  const [resumeFailure, setResumeFailure] = useState<ResumeFailure>(resuming && resumeJobId === null ? 'missing' : null);
  const [seriesId, setSeriesId] = useState<string | null>(null);

  const [fileName, setFileName] = useState<string | null>(null);
  // Bytes of the single .dem sent so far; null once the server holds it.
  const [upload, setUpload] = useState<UploadProgress | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [players, setPlayers] = useState<DemoPlayer[]>([]);
  const [match, setMatch] = useState<RosterMatch | null>(null);

  const [faceitBoard, setFaceitBoard] = useState<{ jobId: string; board: FaceitScoreboard } | null>(null);

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
    setUpload(null);
    setJobId(null);
    setPlayers([]);
    setMatch(null);
    setScanRows([]);
    setParseRows([]);
  }, []);

  const runScan = useCallback(
    async (file: File) => {
      setError(null);
      setWarning(null);
      setFileName(file.name);
      setUpload(uploadProgress(0, file.size));
      setStage('scanning');
      try {
        const scan = await api.scanDemo(file, { onUploadProgress: setUpload });
        setUpload(null);
        if (scan.players.length === 0) {
          reset(DEMO_EMPTY_ROSTER_HINT);
          return;
        }
        setJobId(scan.jobId);
        setPlayers(scan.players);
        setMatch(scan.match ?? null);
        setStage('picking');
      } catch (err) {
        reset(demoScanError(err));
      }
    },
    [reset],
  );

  // The first player leads a multi-POV job; the rest are teammates.
  const onPickPlayers = useCallback(
    async ([steamId, ...teammates]: string[]) => {
      if (stage !== 'picking' || seriesMode || !jobId || !steamId) return;
      setError(null);
      setStage('parsing');
      try {
        const parsed = await api.parseDemo({ jobId, steamId, teammates });
        toast(`Demo analizada: ${prettyMapName(parsed.map)}`, parsed.player ? { description: `${teammates.length > 0 ? 'Jugadores' : 'Jugador'}: ${parsed.player}` } : undefined);
        router.push(produceHref(parsed.id, format));
      } catch (err) {
        setStage('picking');
        setError(demoParseError(err));
      }
    },
    [stage, seriesMode, jobId, router, format],
  );

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
            // One demo's rejection must never sink the others.
            if (isDemoServiceUnavailable(err)) sawOffline = true;
            return { fileName: file.name, status: 'error', reason: demoScanError(err) };
          })
          .then((row) => {
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
        // One shared cause (every demo is CS:GO, say) is worth more than the generic line.
        const reasons = new Set(failed.map((r) => r.reason));
        const shared = reasons.size === 1 ? [...reasons][0] : undefined;
        const allFailed = shared && shared !== DEMO_SCAN_FAIL_HINT
          ? shared
          : 'No se pudo escanear ninguna de las demos. Prueba con otros archivos .dem.';
        reset(sawOffline ? DEMO_SERVICE_OFFLINE_HINT : allFailed);
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

      const finished = await Promise.all(
        rows.map(async (row): Promise<ParseRow> => {
          if (row.status === 'skipped') return row;
          let next: ParseRow;
          try {
            await api.parseDemo({ jobId: row.jobId, steamId });
            next = { ...row, status: 'done' };
          } catch (err) {
            next = { ...row, status: 'error', reason: demoSeriesParseError(err) };
          }
          setParseRows((prev) => prev.map((item) => item.jobId === row.jobId ? next : item));
          return next;
        }),
      );

      if (finished.some((row) => row.status === 'done')) {
        router.push(seriesHref(seriesId, format));
      } else {
        setError('No se pudo completar el análisis de ningún mapa. Consulta el estado de la serie o vuelve a cargar las demos.');
      }
    },
    [stage, seriesMode, seriesId, scannedRows, router, format],
  );

  const onFiles = useCallback(
    (files: File[]) => {
      if (stage !== 'idle' || files.length === 0) return;
      if (files.length === 1) void runScan(files[0]);
      else void runSeriesScan(files);
    },
    [stage, runScan, runSeriesScan],
  );

  // A FACEIT demo shows the room's scoreboard; any other demo, or any failure, keeps the demo's own.
  useEffect(() => {
    if (stage !== 'picking' || seriesMode || jobId === null) return;
    const controller = new AbortController();
    void getFaceitScoreboard(jobId, controller.signal).then((board) => {
      if (!controller.signal.aborted && board !== null) setFaceitBoard({ jobId, board });
    });
    return () => controller.abort();
  }, [stage, seriesMode, jobId]);

  // Imported jobs can still be queued; wait for a roster before offering the player picker.
  useEffect(() => {
    if (resumeJobId === null) return;
    setStage('scanning');
    setResumeFailure(null);
    setJobId(null);
    setPlayers([]);
    setMatch(null);
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        try {
          const scan = await api.getScan(resumeJobId);
          if (!active) return 'idle';
          setResumeFailure(null);
          if (scan === null) {
            setResumeFailure('missing');
            stop();
          } else if (SCAN_PENDING_STATUSES.has(scan.status)) {
            return 'fast';
          } else if (!ROSTER_READY_STATUSES.has(scan.status)) {
            setResumeFailure('error');
            stop();
          } else if (scan.status !== MATCH_STATUS_SCANNED) {
            stop();
            router.replace(produceHref(resumeJobId, format));
          } else if (scan.players.length === 0) {
            setResumeFailure('error');
            stop();
          } else {
            setJobId(resumeJobId);
            setPlayers(scan.players);
            setMatch(scan.match ?? null);
            setStage('picking');
            stop();
          }
        } catch (err: unknown) {
          if (active) setResumeFailure(classifyFullDemoLoadFailure(err));
        }
        return 'idle';
      },
      fastMs: IMPORT_POLL_MS,
      idleMs: IMPORT_RETRY_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [resumeJobId, router, format]);

  /** A resumed job leaves through the URL; an upload in progress only has local state to drop. */
  const leave = useCallback(() => {
    if (resuming) router.push(homeHref);
    else reset(null);
  }, [resuming, router, homeHref, reset]);

  const mapCount = logicalMapGroups.length;
  let title: string = FLOW_HOME_TITLE[format];
  let description: string = FLOW_HOME_DESCRIPTION[format];
  if (resuming) {
    title = PICK_TITLE;
    if (resumeFailure !== null) description = 'No pudimos recuperar los jugadores de esta partida.';
    else if (stage === 'scanning') description = 'Cargando los jugadores de la partida…';
    else description = PICK_DESCRIPTION[format];
  } else if (seriesMode) {
    if (stage === 'scanning') {
      title = 'Escaneando la serie';
      description = `Escaneando ${scanRows.length} demos de la serie…`;
    } else {
      title = seriesTitle(mapCount);
      description =
        stage === 'picking'
          ? `Elige al jugador para buscar sus jugadas en ${scannedRows.map(rowLabel).join(', ')}.`
          : 'Analizando las jugadas del jugador en cada mapa…';
    }
  } else if (stage === 'picking' || stage === 'parsing') {
    title = PICK_TITLE;
    description = PICK_DESCRIPTION[format];
  }

  if (stage === 'parsing') {
    title = error ? 'No se pudo completar el análisis' : 'Analizando las jugadas';
    description = error ? 'Revisa el resultado de cada mapa antes de continuar.' : 'Estamos preparando el contenido del jugador que has elegido.';
  }

  let progressStep = 0;
  // Parsing still belongs to the player step: a failed parse lands back on the picker.
  if (stage === 'picking' || stage === 'parsing' || (stage === 'scanning' && resuming)) progressStep = 1;

  let body: ReactNode;
  if (resumeFailure !== null) {
    const empty = resumeEmptyState(resumeFailure);
    body = (
      <StudioEmptyState
        icon={empty.icon}
        title={empty.title}
        description={empty.description}
        compact
        actions={<Button onClick={() => router.push(homeHref)}>Volver</Button>}
      />
    );
  } else if (stage === 'idle') {
    body = <IdleHome format={format} error={error} onFiles={onFiles} />;
  } else if (seriesMode && stage === 'scanning') {
    body = <ScanRowList rows={scanRows} />;
  } else if (seriesMode && stage === 'parsing') {
    body = (
      <div className="flex flex-col gap-3">
        <ParseRowList rows={parseRows} />
        {error ? (
          <>
            <ErrorBanner message={error} />
            <div className="flex flex-wrap gap-3">
              <Button onClick={() => reset(null)}>Volver a cargar demos</Button>
              <Button variant="outline" onClick={() => seriesId && router.push(seriesHref(seriesId, format))}>Ver estado de la serie</Button>
            </div>
          </>
        ) : null}
      </div>
    );
  } else if (seriesMode) {
    body = <PlayerPicker players={aggregated} purpose={format === PRODUCE_FORMAT.full ? 'full-demo' : 'highlights'} allowDestinationSwitch={false} onPick={onPickSeries} seriesMapCount={mapCount} onCancel={leave} tone={format} />;
  } else if (stage === 'scanning') {
    body = <SingleDemoProgress label={resuming ? 'Cargando jugadores…' : 'Buscando jugadores…'} fileName={fileName} upload={upload} />;
  } else if (stage === 'parsing') {
    body = <SingleDemoProgress label="Extrayendo las rondas y jugadas de la partida…" fileName={fileName} />;
  } else {
    body = (
      <div className="flex flex-col gap-3">
        {error ? <ErrorBanner message={error} /> : null}
        <PlayerPicker players={players} purpose={format === PRODUCE_FORMAT.full ? 'full-demo' : 'highlights'} tone={format} allowDestinationSwitch={false} onPick={(steamId) => onPickPlayers([steamId])} onPickMany={onPickPlayers} match={match ?? undefined} onCancel={leave} faceit={faceitBoard?.jobId === jobId ? faceitBoard.board : null} />
      </div>
    );
  }

  const atHome = stage === 'idle' && !resuming;
  return (
    <div className="measure-work flex flex-col gap-6">
      {atHome ? <FlowHomeHero format={format} /> : (
        <>
          <StudioBackLink onClick={leave}>{FLOW_TITLE[format]}</StudioBackLink>
          <header className="flex flex-wrap items-end gap-x-8 gap-y-4">
            <div className="flex min-w-0 flex-[1_1_26rem] flex-col gap-2.5">
              <h1 className="text-balance font-display text-display font-extrabold leading-[1.05] tracking-[-0.025em] text-fg-1">{title}</h1>
              <p className="text-body text-fg-2">{description}</p>
              {stage === 'picking' && !seriesMode && resumeFailure === null ? <ScannedDemoLine fileName={fileName} match={match} withScore={faceitBoard?.jobId === jobId} /> : null}
            </div>
            <CreateFlowSteps flow={format} step={progressStep} className="w-120 max-w-full" />
          </header>
          {seriesMode ? <p className="text-body-sm text-fg-2">{SERIES_NOTE}</p> : null}
        </>
      )}
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
            className="grid size-10 shrink-0 place-items-center transition-colors duration-(--dur-instant) hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-warning"
          >
            <X className="size-4" />
          </button>
        </div>
      ) : null}
      {body}
    </div>
  );
}

type IdleHomeProps = {
  format: ProduceFormat;
  error: string | null;
  onFiles: (files: File[]) => void;
};

/** The upload home. It mounts fresh on every return to idle, so the demo count starts unknown each time. */
function IdleHome({ format, error, onFiles }: IdleHomeProps): ReactNode {
  // null until the list's first read settles; the upload waits for it so the page lays out once.
  const [demoCount, setDemoCount] = useState<number | null>(null);
  const [source, setSource] = useState<SourceKind>(SOURCE_KIND.file);
  const hasDemos = demoCount !== null && demoCount > 0;
  const steamOpen = source === SOURCE_KIND.steam;
  const steamToggle = (
    <Button type="button" variant={hasDemos ? 'ghost' : 'outline'} size="sm" aria-expanded={steamOpen} aria-controls="steam-import"
      onClick={() => setSource(steamOpen ? SOURCE_KIND.file : SOURCE_KIND.steam)}>
      Importar desde Steam
    </Button>
  );
  const addTile = <DemoDropzone onFiles={onFiles} minHeightClass="min-h-62.5" tile={{ tone: format, title: 'Suelta aquí otra demo', actions: steamToggle }} />;
  return (
    <div className="flex flex-col gap-7">
      <DemoList format={format} onCount={setDemoCount} addTile={addTile} />
      {hasDemos ? (
        <details className="group/source -mt-3 self-start">
          <summary className={cn('flex min-h-9 cursor-pointer list-none items-center gap-1.5 rounded-md px-2 text-body-sm font-semibold text-fg-2 hover:text-fg-1 [&::-webkit-details-marker]:hidden', FOCUS_RING)}>
            ¿Dónde consigo una demo?
            <ChevronDown aria-hidden className="size-4 shrink-0 transition-transform group-open/source:rotate-180" />
          </summary>
          <div className="mt-2 max-w-md">
            <DemoSourceHelp titled={false} />
          </div>
        </details>
      ) : null}
      {demoCount === null ? (
        <div role="status" aria-label="Cargando demos" className="grid grid-cols-[repeat(auto-fill,minmax(17.5rem,1fr))] gap-4">
          <Skeleton className="h-62.5 rounded-2xl" />
          <Skeleton className="h-62.5 rounded-2xl" />
          <Skeleton className="h-62.5 rounded-2xl" />
        </div>
      ) : null}
      {demoCount === 0 ? (
        <section aria-label="Tu primera demo" className="grid gap-4 @[48rem]/content:grid-cols-[minmax(0,1.4fr)_minmax(16rem,1fr)]">
          <DemoDropzone onFiles={onFiles} minHeightClass="min-h-80" tile={{ tone: format, title: 'Suelta aquí tu demo de CS2', strong: true, actions: steamToggle }} />
          <DemoSourceHelp />
        </section>
      ) : null}
      {error ? <ErrorBanner message={error} /> : null}
      {steamOpen ? (
        <section id="steam-import" aria-labelledby="steam-import-title" className="studio-panel flex flex-col gap-4 rounded-2xl p-4 @[40rem]/content:p-6">
          <div className="flex items-center justify-between gap-3">
            <h2 id="steam-import-title" className="font-display text-title font-bold text-fg-1">Importar desde Steam</h2>
            <Button type="button" variant="ghost" size="icon-sm" aria-label="Cerrar la importación desde Steam" onClick={() => setSource(SOURCE_KIND.file)}>
              <X aria-hidden />
            </Button>
          </div>
          <ShareCodeDoor format={format} />
          <RecentSteamMatches format={format} />
        </section>
      ) : null}
    </div>
  );
}

function resumeEmptyState(
  failure: Exclude<ResumeFailure, null>,
): { icon: typeof SearchX; title: string; description: string } {
  if (failure === 'offline') return { icon: Unplug, ...fullDemoEmptyState(failure) };
  if (failure === 'error') return { icon: AlertTriangle, ...fullDemoEmptyState(failure) };
  return { icon: SearchX, ...PRODUCE_MATCH_MISSING };
}

function ErrorBanner({ message }: { message: string }): ReactNode {
  return (
    <p role="alert" className="flex items-start gap-2.5 border border-destructive/45 bg-destructive/8 px-4 py-3 text-body-sm text-destructive">
      <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
      {message}
    </p>
  );
}

/** The scanned demo under the pick guidance. The score shows here only when the picker hides its own match header (FACEIT). */
function ScannedDemoLine({ fileName, match, withScore }: { fileName: string | null; match: RosterMatch | null; withScore: boolean }): ReactNode {
  const line = withScore ? matchLine(match) : null;
  if (fileName === null && line === null) return null;
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-1.5 text-body-sm text-fg-2">
      <span aria-hidden className="h-6 w-10.5 shrink-0 overflow-hidden rounded-md">
        <MapCover map={match?.map ?? ''} />
      </span>
      {fileName === null ? null : <span className="min-w-0 truncate font-mono text-fg-1">{fileName}</span>}
      {line === null ? null : <span className="tabular-nums">{line}</span>}
    </div>
  );
}

function SingleDemoProgress({ label, fileName, upload = null }: { label: string; fileName: string | null; upload?: UploadProgress | null }): ReactNode {
  const uploading = upload !== null && !uploadSent(upload);
  return (
    <div role={uploading ? undefined : 'status'} aria-live={uploading ? undefined : 'polite'} className="flex min-h-[360px] flex-col items-center justify-center gap-4 text-center">
      <span className="grid size-14 place-items-center border border-border-accent bg-surface-0 text-fg-2">
        <span aria-hidden className="studio-spinner size-6" />
      </span>
      <p className="font-display text-title font-bold text-fg-1">{uploading ? 'Subiendo la demo…' : label}</p>
      {fileName ? (
        <p className="inline-flex max-w-full items-center gap-2 font-mono text-body-sm text-fg-2">
          <FileVideo aria-hidden className="size-4 shrink-0" />
          <span className="truncate">{fileName}</span>
        </p>
      ) : null}
      {uploading ? (
        <LongOperation stage="Subiendo" percent={upload.percent} detail={formatUploadBytes(upload)} className="w-full max-w-80 text-left" />
      ) : (
        <span className="studio-bar w-full max-w-80 text-primary">
          <span className="studio-indeterminate" />
        </span>
      )}
    </div>
  );
}

function ScanRowList({ rows }: { rows: ScanRow[] }): ReactNode {
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-2">
      <p className="mb-1 text-meta text-fg-3">Escaneando {rows.length} demos</p>
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
      <StatusTag tone="success" icon={CheckCircle2}>
        {row.match ? prettyMapName(row.match.map) : 'Escaneada'}
      </StatusTag>
    );
  }
  return (
    <StatusTag tone="danger" icon={AlertTriangle} className="max-w-[11rem] @[40rem]/content:max-w-xs">
      <span className="min-w-0 truncate" title={row.reason}>
        {row.reason ?? 'Error'}
      </span>
    </StatusTag>
  );
}

function ParseRowList({ rows }: { rows: ParseRow[] }): ReactNode {
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-2">
      <p className="mb-1 text-meta text-fg-3">
        {rows.some((row) => row.status === 'parsing') ? 'Analizando las jugadas en cada mapa' : 'Resultado por mapa'}
      </p>
      {rows.map((row) => (
        <div key={row.jobId} className="flex flex-col gap-1">
          <StudioDataRow
            active={row.status === 'parsing'}
            label={row.label}
            status={<ParseRowStatus status={row.status} />}
          />
          {row.reason ? <p className="px-3 text-body-sm text-destructive">{row.reason}</p> : null}
        </div>
      ))}
    </div>
  );
}

function ParseRowStatus({ status }: { status: ParseRow['status'] }): ReactNode {
  switch (status) {
    case 'parsing':
      return (
        <StatusTag icon={Loader2} className="[&_svg]:animate-spin">
          Analizando
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
