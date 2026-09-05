'use client';

import { use, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, CheckCircle2, FileVideo, Loader2, SearchX, Unplug, X } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { DEMO_CREATION_STEPS } from '@/lib/clips/copy';
import type { DemoPlayer, RosterMatch } from '@/lib/api/types';
import { aggregateGroupedSeriesRoster } from '@/lib/api/series-roster';
import { MATCH_STATUS_SCANNED } from '@/lib/clips/hub';
import { takePendingDemoFiles } from '@/lib/clips/pending-upload';
import {
  CLIPS_HREF,
  isJobIdParam,
  isProduceFormat,
  PRODUCE_QUERY,
  newDemoHref,
  NEW_DEMO_QUERY,
  PRODUCE_FORMAT,
  produceHref,
  seriesHref,
} from '@/lib/clips/routes';
import {
  DEMO_EMPTY_ROSTER_HINT,
  DEMO_SERVICE_OFFLINE_HINT,
  demoParseError,
  demoScanError,
  isDemoServiceUnavailable,
} from '@/lib/demo-parse-flow';
import { startPollLoop } from '@/lib/poll-loop';
import { ROSTER_READY_STATUSES, SCAN_PENDING_STATUSES } from '@/lib/api/types';
import { prettyMapName } from '@/lib/format';
import { classifyFullDemoLoadFailure, fullDemoEmptyState, type FullDemoLoadFailure } from '@/lib/full-demo';
import { PRODUCE_MATCH_MISSING } from '@/lib/produce/copy';
import { groupSeriesDemos } from '@/lib/series-grouping';
import { seriesTitle } from '@/lib/series-status';
import { MapCover } from '@/components/brand/map-cover';
import { DemoSourceHelp } from '@/components/onboarding/demo-source-help';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { RecentSteamMatches } from '@/components/onboarding/recent-matches';
import { ShareCodeDoor } from '@/components/onboarding/share-code-door';
import { StatusTag } from '@/components/studio/status-tag';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StudioBackLink } from '@/components/studio/back-link';
import { WorkflowProgress } from '@/components/studio/workflow-progress';
import { ProduceFormatBar } from '@/components/produce/format-bar';
import { StudioDataRow } from '@/components/studio/data-row';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
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

type ParseRow = { jobId: string; label: string; status: 'parsing' | 'done' | 'skipped' | 'error' };

const SOURCE_KIND = { file: 'file', steam: 'steam' } as const;

const IMPORT_POLL_MS = 1500;
const IMPORT_RETRY_MS = 10000;

const ZERO_PLAYERS_HINT = 'Sin jugadores — ¿seguro que es una demo de CS2?';

function rowLabel(row: Extract<ScanRow, { status: 'scanned' }>): string {
  return row.match ? prettyMapName(row.match.map) : row.fileName;
}

/**
 * Cargar demo → escanear → elegir POV → parsear; series bo3/bo5 walk the same page.
 * `?job=` skips the upload and resumes a `scanned` job at the same picker.
 */
export default function NewDemoPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}): ReactNode {
  const router = useRouter();
  const query = use(searchParams);
  const jobParam = query[NEW_DEMO_QUERY.job];
  const formatParam = query[PRODUCE_QUERY.format];
  const format = typeof formatParam === 'string' && isProduceFormat(formatParam) ? formatParam : PRODUCE_FORMAT.short;
  const resuming = jobParam !== undefined;
  const resumeJobId = isJobIdParam(jobParam) ? jobParam : null;

  const [stage, setStage] = useState<Stage>(resuming ? 'scanning' : 'idle');
  const [resumeFailure, setResumeFailure] = useState<ResumeFailure>(resuming && resumeJobId === null ? 'missing' : null);
  const [seriesId, setSeriesId] = useState<string | null>(null);

  const [fileName, setFileName] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [players, setPlayers] = useState<DemoPlayer[]>([]);
  const [match, setMatch] = useState<RosterMatch | null>(null);

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

  const runScan = useCallback(
    async (file: File) => {
      setError(null);
      setWarning(null);
      setFileName(file.name);
      setStage('scanning');
      try {
        const scan = await api.scanDemo(file);
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

  const onPickSingle = useCallback(
    async (steamId: string) => {
      if (stage !== 'picking' || seriesMode || !jobId) return;
      setError(null);
      setStage('parsing');
      try {
        const parsed = await api.parseDemo({ jobId, steamId });
        toast(`Demo analizada: ${prettyMapName(parsed.map)}`, { description: `Jugador: ${parsed.player ?? '—'}` });
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
            return { fileName: file.name, status: 'error' };
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
        reset(sawOffline ? DEMO_SERVICE_OFFLINE_HINT : 'No se pudo escanear ninguna de las demos. Prueba con otros archivos .dem.');
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

      router.push(seriesHref(seriesId));
    },
    [stage, seriesMode, seriesId, scannedRows, router],
  );

  const onFiles = useCallback(
    (files: File[]) => {
      if (stage !== 'idle' || files.length === 0) return;
      if (files.length === 1) void runScan(files[0]);
      else void runSeriesScan(files);
    },
    [stage, runScan, runSeriesScan],
  );

  // The hub's empty-state dropzone parks its files here; `take` empties the store, so re-runs are no-ops.
  useEffect(() => {
    const handed = takePendingDemoFiles();
    if (handed.length > 0) onFiles(handed);
  }, [onFiles]);

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

  const mapCount = logicalMapGroups.length;
  let title = format === PRODUCE_FORMAT.full ? 'Crea un vídeo largo' : 'Crea un Short';
  let description = 'Carga una demo de CS2 y elige el jugador. Después revisarás el contenido y los ajustes antes de grabar.';
  if (resuming) {
    title = 'Elige el jugador';
    if (resumeFailure !== null) description = 'No pudimos recuperar los jugadores de esta partida.';
    else if (stage === 'scanning') description = 'Cargando los jugadores de la partida…';
    else description = 'Elige de quién será el vídeo. Después podrás revisar el contenido antes de crearlo.';
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
    title = 'Elige el jugador';
    description = 'Elige de quién será el vídeo. Después podrás revisar el contenido antes de crearlo.';
  }

  let progressStep = 0;
  if (stage === 'picking') progressStep = 1;
  if (stage === 'parsing') progressStep = 1;

  let body: ReactNode;
  if (resumeFailure !== null) {
    const empty = resumeEmptyState(resumeFailure);
    body = (
      <StudioEmptyState
        icon={empty.icon}
        title={empty.title}
        description={empty.description}
        compact
        actions={<Button onClick={() => router.push(CLIPS_HREF)}>Volver</Button>}
      />
    );
  } else if (stage === 'idle') {
    body = (
      <div className="flex flex-col gap-5">
        <Tabs defaultValue={SOURCE_KIND.file} className="gap-4">
          <TabsList aria-label="Origen de la demo" className="h-auto! min-h-11 w-full sm:w-fit">
            <TabsTrigger value={SOURCE_KIND.file} className="min-h-10">Archivo en mi PC</TabsTrigger>
            <TabsTrigger value={SOURCE_KIND.steam} className="min-h-10">Importar desde Steam</TabsTrigger>
          </TabsList>
          <TabsContent value={SOURCE_KIND.file} className="flex flex-col gap-3">
            <DemoDropzone onFiles={onFiles} minHeightClass="min-h-[200px]" />
            {error ? <ErrorBanner message={error} /> : null}
            <DemoSourceHelp />
          </TabsContent>
          <TabsContent value={SOURCE_KIND.steam} className="flex flex-col gap-4">
            <ShareCodeDoor format={format} />
            <RecentSteamMatches format={format} />
          </TabsContent>
        </Tabs>
      </div>
    );
  } else if (seriesMode && stage === 'scanning') {
    body = <ScanRowList rows={scanRows} />;
  } else if (seriesMode && stage === 'parsing') {
    body = <ParseRowList rows={parseRows} />;
  } else if (seriesMode) {
    body = (
      <Card className="studio-panel-raised p-4 @[40rem]/content:p-6">
        <PlayerPicker players={aggregated} onPick={onPickSeries} seriesMapCount={mapCount} cancelHref={CLIPS_HREF} />
      </Card>
    );
  } else if (stage === 'scanning') {
    body = <SingleDemoProgress label={resuming ? 'Cargando jugadores…' : 'Buscando jugadores…'} fileName={fileName} />;
  } else if (stage === 'parsing') {
    body = <SingleDemoProgress label="Analizando las jugadas…" fileName={fileName} />;
  } else {
    body = (
      <div className="flex flex-col gap-3">
        <ScannedDemoRow fileName={fileName} match={match} />
        {error ? <ErrorBanner message={error} /> : null}
        <Card className="studio-panel-raised p-4 @[40rem]/content:p-6">
          <PlayerPicker players={players} purpose={format === PRODUCE_FORMAT.full ? 'full-demo' : 'highlights'} allowDestinationSwitch={false} onPick={onPickSingle} match={match ?? undefined} cancelHref={CLIPS_HREF} />
        </Card>
      </div>
    );
  }

  return (
    <div className="measure-list flex flex-col gap-5">
      <StudioBackLink href={CLIPS_HREF}>Demos y vídeos</StudioBackLink>
      <StudioPageHeader title={title} description={description} />
      <WorkflowProgress steps={DEMO_CREATION_STEPS} current={progressStep} />
      {stage === 'idle' && !resuming ? (
        <ProduceFormatBar value={format} onChange={(next) => router.replace(newDemoHref({ format: next }), { scroll: false })} />
      ) : null}
      {seriesMode ? <p className="text-body-sm text-fg-2">En una serie elegirás Short o vídeo largo para cada mapa después del análisis.</p> : null}
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

/** The scanned demo above the picker: cover, map + score, file, source, "Escaneada". */
function ScannedDemoRow({ fileName, match }: { fileName: string | null; match: RosterMatch | null }): ReactNode {
  return (
    <div className="studio-panel studio-enter flex items-center gap-4 rounded-[10px] px-4 py-3">
      <span aria-hidden className="h-[47px] w-[84px] shrink-0 overflow-hidden border border-border-strong">
        <MapCover map={match?.map ?? ''} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate font-display text-body-lg font-bold uppercase text-fg-1">
          {match ? (
            <>
              {prettyMapName(match.map)}{' '}
              <span className={match.scoreT > match.scoreCt ? 'text-warning' : 'text-fg-3'}>{match.scoreT}</span>
              <span className="text-fg-3"> : </span>
              <span className={match.scoreCt > match.scoreT ? 'text-primary' : 'text-fg-3'}>{match.scoreCt}</span>
            </>
          ) : (
            'Demo escaneada'
          )}
        </span>
        <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
          {fileName ?? 'demo'} · {match ? `${match.rounds} rondas` : '—'}
        </span>
      </span>
      <StatusTag tone="success" dot>
        Escaneada
      </StatusTag>
    </div>
  );
}

function SingleDemoProgress({ label, fileName }: { label: string; fileName: string | null }): ReactNode {
  return (
    <div role="status" aria-live="polite" className="studio-enter flex min-h-[360px] flex-col items-center justify-center gap-4 text-center">
      <span className="grid size-14 place-items-center border border-border-accent bg-surface-0 text-primary shadow-[var(--glow-primary-md)]">
        <span aria-hidden className="studio-spinner size-6" />
      </span>
      <p className="font-display text-title font-bold uppercase text-fg-1">{label}</p>
      {fileName ? (
        <p className="inline-flex max-w-full items-center gap-2 font-mono text-body-sm text-fg-2">
          <FileVideo aria-hidden className="size-4 shrink-0" />
          <span className="truncate">{fileName}</span>
        </p>
      ) : null}
      <span className="studio-bar w-full max-w-80 text-primary">
        <span className="studio-indeterminate" />
      </span>
    </div>
  );
}

function ScanRowList({ rows }: { rows: ScanRow[] }): ReactNode {
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
      <p className="mb-1 font-mono text-meta uppercase tracking-wider text-fg-3">Analizando las jugadas en cada mapa</p>
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
