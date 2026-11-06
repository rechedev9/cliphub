'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { ChevronDown, Download } from 'lucide-react';
import { streamsApi, type StreamPlay, type StreamSummary } from '@/lib/api/streams';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { sameIds, summaryPlayIds } from '@/lib/streams/plays';
import { Button, buttonVariants } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';

const POLL_MS = 2000;
const FAIL_MESSAGE = 'No pudimos crear el resumen. Vuelve a intentarlo.';

/** One landscape video with the plays the user picks, in source order. */
export function StreamSummaryCard({
  jobId,
  plays,
  disabled,
}: {
  jobId: string;
  plays: readonly StreamPlay[];
  disabled: boolean;
}): ReactNode {
  const [summary, setSummary] = useState<StreamSummary | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [chosen, setChosen] = useState<ReadonlySet<string> | null>(null);
  const [picking, setPicking] = useState(false);
  const running = summary?.status === 'running';

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const next = await streamsApi.getSummary(jobId);
        if (active) setSummary(next);
      } catch (err) {
        if (active) setRequestError(errorMessage(err, FAIL_MESSAGE));
      }
    };
    void load();
    const timer = running ? setInterval(() => void load(), POLL_MS) : undefined;
    return () => {
      active = false;
      if (timer) clearInterval(timer);
    };
  }, [jobId, running]);

  // The last summary's plays seed the choice; with none, every play is in.
  const covered = summary ? summaryPlayIds(summary, plays) : null;
  const selected = chosen ?? new Set(covered ?? plays.map((play) => play.id));
  const selectedPlays = plays.filter((play) => selected.has(play.id));
  const selectedSeconds = selectedPlays.reduce((total, play) => total + play.end_seconds - play.start_seconds, 0);

  const toggle = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setChosen(next);
  };

  const start = async () => {
    setRequestError(null);
    try {
      setSummary(await streamsApi.startSummary(jobId, selectedPlays.map((play) => play.id)));
    } catch (err) {
      setRequestError(errorMessage(err, FAIL_MESSAGE));
    }
  };

  const done = summary?.status === 'done';
  // A summary of other plays (a new scan or a new choice) can be remade.
  const stale = done && (covered === null || !sameIds(covered, selectedPlays.map((play) => play.id)));
  const videoUrl = done ? streamsApi.summaryVideoUrl(jobId, summary.rendered_at) : null;
  const canStart = !disabled && selectedPlays.length > 0;
  const allChosen = selectedPlays.length === plays.length;

  const picker = (
    <div className="flex flex-col gap-2">
      <button
        type="button"
        aria-expanded={picking}
        className="flex min-h-11 items-center justify-between gap-2 rounded text-left text-body-sm text-fg-2 hover:text-fg-1"
        onClick={() => setPicking((open) => !open)}
      >
        <span>
          Jugadas del resumen · <span className="tabular-nums">{selectedPlays.length} de {plays.length}</span>
        </span>
        <ChevronDown aria-hidden className={picking ? 'size-4 rotate-180' : 'size-4'} />
      </button>
      {picking ? (
        <div className="flex flex-col gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="self-start"
            disabled={disabled}
            onClick={() => setChosen(new Set(allChosen ? [] : plays.map((play) => play.id)))}
          >
            {allChosen ? 'Quitar todas' : 'Elegir todas'}
          </Button>
          <ul className="flex max-h-64 flex-col overflow-y-auto">
            {plays.map((play) => (
              <li key={play.id}>
                <label className="flex min-h-11 cursor-pointer items-center gap-3 text-body-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={selected.has(play.id)}
                    disabled={disabled}
                    onChange={() => toggle(play.id)}
                  />
                  <span className="font-semibold">{play.label}</span>
                  <span className="text-fg-3 tabular-nums">
                    {formatStreamClock(play.kill_seconds[0] ?? play.start_seconds)}
                  </span>
                </label>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );

  return (
    <section aria-label="Resumen horizontal" className="flex flex-col gap-3 rounded-md border border-border-subtle p-3">
      <h3 className="font-semibold">Resumen horizontal</h3>

      {summary === null && !requestError ? <p className="text-body-sm text-fg-3">Comprobando…</p> : null}

      {done && videoUrl ? (
        <div className="flex flex-col gap-2">
          <video src={videoUrl} controls preload="metadata" className="aspect-video w-full rounded bg-black" />
          <p className="text-body-sm text-fg-2 tabular-nums">
            {summary.play_count} jugadas · {formatStreamClock(summary.duration_seconds ?? 0)}
          </p>
          <a href={videoUrl} download="resumen.mp4" className={buttonVariants({ variant: 'outline' })}>
            <Download aria-hidden />
            Guardar resumen
          </a>
        </div>
      ) : null}

      {summary !== null && !running ? picker : null}

      {summary?.status === 'idle' || stale ? (
        <>
          <p className="text-body-sm text-fg-2">
            {selectedPlays.length === 0
              ? 'Elige al menos una jugada.'
              : `Un vídeo 16:9 con ${allChosen ? `tus ${plays.length}` : `${selectedPlays.length} de tus ${plays.length}`} jugadas seguidas, unos ${formatStreamClock(selectedSeconds)}.`}
          </p>
          <Button variant="outline" disabled={!canStart} onClick={() => void start()}>
            {stale ? 'Rehacer resumen' : 'Crear resumen'}
          </Button>
        </>
      ) : null}

      {running ? (
        <div className="flex flex-col gap-2">
          <p className="text-body-sm text-fg-2" aria-live="polite">
            Creando resumen · {Math.round((summary.progress ?? 0) * 100)} %
          </p>
          <Progress tone="stream" size="sm" value={(summary.progress ?? 0) * 100} />
        </div>
      ) : null}

      {summary?.status === 'failed' || requestError ? (
        <div className="flex flex-col gap-2">
          <p role="alert" className="text-body-sm text-destructive">
            {requestError ?? summary?.error ?? FAIL_MESSAGE}
          </p>
          <Button variant="outline" disabled={!canStart} onClick={() => void start()}>
            Volver a intentar
          </Button>
        </div>
      ) : null}
    </section>
  );
}
