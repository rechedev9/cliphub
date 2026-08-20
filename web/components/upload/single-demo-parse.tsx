'use client';

import { useCallback, useState, type ReactNode } from 'react';
import { AlertTriangle, FileVideo, Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { DemoPlayer, Match, RosterMatch } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { Card } from '@/components/ui/card';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { PlayerPicker } from '@/components/upload/player-picker';

type Stage = 'idle' | 'scanning' | 'picking' | 'parsing';

const OFFLINE_HINT = 'El servicio de análisis está offline. Arráncalo y vuelve a intentarlo.';
const SCAN_FAIL_HINT = 'No se pudo escanear esa demo. Prueba con otro archivo .dem.';
const EMPTY_ROSTER_HINT =
  'El escaneo no encontró jugadores en esa demo. ¿Seguro que es una demo de CS2? Prueba con otro archivo .dem.';
const PARSE_FAIL_HINT = 'No se pudieron extraer los highlights de ese jugador. Elige otro.';
const SINGLE_DEMO_HINT = 'Esta sección forja una partida. Suelta un solo .dem.';

function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

export type SingleDemoParseProps = {
  onParsed: (match: Match) => void;
};

export function SingleDemoParse({ onParsed }: SingleDemoParseProps): ReactNode {
  const [stage, setStage] = useState<Stage>('idle');
  const [fileName, setFileName] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [players, setPlayers] = useState<DemoPlayer[]>([]);
  const [match, setMatch] = useState<RosterMatch | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reset = useCallback((message: string | null) => {
    setError(message);
    setStage('idle');
    setFileName(null);
    setJobId(null);
    setPlayers([]);
    setMatch(null);
  }, []);

  const runScan = useCallback(
    async (file: File) => {
      setError(null);
      setFileName(file.name);
      setStage('scanning');
      try {
        const scan = await api.scanDemo(file);
        if (scan.players.length === 0) {
          reset(EMPTY_ROSTER_HINT);
          return;
        }
        setJobId(scan.jobId);
        setPlayers(scan.players);
        setMatch(scan.match ?? null);
        setStage('picking');
      } catch (err) {
        reset(isServiceUnavailable(err) ? OFFLINE_HINT : SCAN_FAIL_HINT);
      }
    },
    [reset],
  );

  const onPick = useCallback(
    async (steamId: string) => {
      if (stage !== 'picking' || !jobId) return;
      setError(null);
      setStage('parsing');
      try {
        onParsed(await api.parseDemo({ jobId, steamId }));
      } catch (err) {
        reset(isServiceUnavailable(err) ? OFFLINE_HINT : PARSE_FAIL_HINT);
      }
    },
    [stage, jobId, onParsed, reset],
  );

  const onFiles = useCallback(
    (files: File[]) => {
      if (stage !== 'idle' || files.length === 0) return;
      const file = files[0];
      if (files.length !== 1 || !file) {
        setError(SINGLE_DEMO_HINT);
        return;
      }
      void runScan(file);
    },
    [stage, runScan],
  );

  let card: ReactNode = null;
  if (stage === 'scanning' || stage === 'parsing') {
    card = <ScanProgress stage={stage} fileName={fileName} />;
  } else if (stage === 'picking') {
    card = <PlayerPicker players={players} onPick={onPick} match={match ?? undefined} />;
  }

  return (
    <div className="@container/upload flex flex-col gap-3">
      {error ? (
        <p
          role="alert"
          className="flex items-start gap-2.5 border border-destructive/45 bg-destructive/8 px-4 py-3 text-body-sm text-destructive"
        >
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {error}
        </p>
      ) : null}
      {stage === 'idle' ? (
        <DemoDropzone onFiles={onFiles} />
      ) : (
        <Card className="studio-panel-raised p-4 @[40rem]/upload:p-6">{card}</Card>
      )}
    </div>
  );
}

function ScanProgress({ stage, fileName }: { stage: 'scanning' | 'parsing'; fileName: string | null }): ReactNode {
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
