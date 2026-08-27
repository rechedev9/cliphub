'use client';

import { useCallback, useState, type ReactNode } from 'react';
import { AlertTriangle, FileVideo, Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { DemoPlayer, Match, RosterMatch } from '@/lib/api/types';
import {
  DEMO_EMPTY_ROSTER_HINT,
  DEMO_SINGLE_FILE_HINT,
  demoParseError,
  demoScanError,
} from '@/lib/demo-parse-flow';
import { Card } from '@/components/ui/card';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { PlayerPicker } from '@/components/upload/player-picker';

type Stage = 'idle' | 'scanning' | 'picking' | 'parsing';

export type SingleDemoParseProps = {
  onParsed: (match: Match) => void;
  purpose?: 'highlights' | 'full-demo';
};

export function SingleDemoParse({ onParsed, purpose = 'highlights' }: SingleDemoParseProps): ReactNode {
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

  const onPick = useCallback(
    async (steamId: string) => {
      if (stage !== 'picking' || !jobId) return;
      setError(null);
      setStage('parsing');
      try {
        onParsed(await api.parseDemo({ jobId, steamId }));
      } catch (err) {
        setError(demoParseError(err));
        setStage('picking');
      }
    },
    [stage, jobId, onParsed],
  );

  const onFiles = useCallback(
    (files: File[]) => {
      if (stage !== 'idle' || files.length === 0) return;
      const file = files[0];
      if (files.length !== 1 || !file) {
        setError(DEMO_SINGLE_FILE_HINT);
        return;
      }
      void runScan(file);
    },
    [stage, runScan],
  );

  let card: ReactNode = null;
  if (stage === 'scanning' || stage === 'parsing') {
    card = <ScanProgress stage={stage} fileName={fileName} purpose={purpose} />;
  } else if (stage === 'picking') {
    card = <PlayerPicker players={players} onPick={onPick} match={match ?? undefined} purpose={purpose} />;
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

function ScanProgress({
  stage,
  fileName,
  purpose,
}: {
  stage: 'scanning' | 'parsing';
  fileName: string | null;
  purpose: 'highlights' | 'full-demo';
}): ReactNode {
  let statusLabel = 'Escaneando el roster…';
  if (stage === 'parsing') {
    statusLabel = purpose === 'full-demo' ? 'Preparando las rondas…' : 'Forjando highlights…';
  }

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
          {statusLabel}
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
