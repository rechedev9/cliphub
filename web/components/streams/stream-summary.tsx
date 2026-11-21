'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Download, Mic } from 'lucide-react';
import { streamsApi, type StreamPlay, type StreamSummary } from '@/lib/api/streams';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { isVoiceOnly, orderPlays, sameIds, summaryPlayIds } from '@/lib/streams/plays';
import { playTimeLabel } from '@/lib/streams/moments';
import { SUMMARY_VIDEO } from '@/lib/streams/subtitles';
import { summaryBulkChoice, summaryChoiceText, summaryDefaultIds } from '@/lib/streams/summary';
import { clipStretch, StreamSubtitlesPanel } from '@/components/streams/stream-subtitles-panel';
import { Button, buttonVariants } from '@/components/ui/button';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';
import { Progress } from '@/components/ui/progress';

const POLL_MS = 2000;
const FAIL_MESSAGE = 'No pudimos crear el resumen. Vuelve a intentarlo.';
const POSTER_SECONDS = 1.5;

function startLabel(status: StreamSummary['status'], stale: boolean): string {
  if (status === 'failed') return 'Volver a intentar';
  return stale ? 'Rehacer resumen' : 'Crear resumen';
}

function SummaryVideo({ src }: { src: string }): ReactNode {
  const poster = useRef<number | null>(null);
  return (
    <video
      src={src}
      aria-label="Resumen"
      controls
      preload="metadata"
      className="aspect-video w-full rounded bg-black"
      onLoadedMetadata={(event) => {
        const video = event.currentTarget;
        if (video.currentTime !== 0 || !Number.isFinite(video.duration)) return;
        poster.current = Math.min(POSTER_SECONDS, video.duration / 2);
        video.currentTime = poster.current;
      }}
      onPlay={(event) => {
        const video = event.currentTarget;
        if (poster.current !== null && Math.abs(video.currentTime - poster.current) < 0.05) video.currentTime = 0;
        poster.current = null;
      }}
    />
  );
}

export function StreamSummaryCard({ jobId, plays }: { jobId: string; plays: readonly StreamPlay[] }): ReactNode {
  const [summary, setSummary] = useState<StreamSummary | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [chosen, setChosen] = useState<ReadonlySet<string> | null>(null);
  const [subtitlesStale, setSubtitlesStale] = useState(false);
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

  const covered = summary ? summaryPlayIds(summary, plays) : null;
  const selected = chosen ?? new Set(covered ?? summaryDefaultIds(plays));
  const ordered = orderPlays(plays, 'time');
  const selectedPlays = ordered.filter((play) => selected.has(play.id));
  const bulk = summaryBulkChoice(selected, plays);
  const hasKills = plays.some((play) => !isVoiceOnly(play));

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
  const stale = done && (covered === null || !sameIds(covered, selectedPlays.map((play) => play.id)) || subtitlesStale);
  const videoUrl = done ? streamsApi.summaryVideoUrl(jobId, summary.rendered_at) : null;
  const canStart = !running && selectedPlays.length > 0;
  const stretches = selectedPlays.map(clipStretch);

  return (
    <div className="grid gap-5 @[56rem]/content:grid-cols-[minmax(0,3fr)_minmax(18rem,2fr)]">
      <section aria-label="Tu resumen" className="flex min-w-0 flex-col gap-3">
        {summary === null && !requestError ? <p className="text-body-sm text-fg-3">Comprobando…</p> : null}

        {done && videoUrl ? (
          <>
            <SummaryVideo key={videoUrl} src={videoUrl} />
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-body-sm text-fg-2 tabular-nums">
                {summary.play_count} jugadas · {formatStreamClock(summary.duration_seconds ?? 0)}
              </p>
              <a href={videoUrl} download="resumen.mp4" className={buttonVariants({ variant: 'outline' })}>
                <Download aria-hidden />
                Guardar resumen
              </a>
            </div>
            {summary.subtitles_notice ? <p role="status" className="text-body-sm text-warning">{summary.subtitles_notice}</p> : null}
            {subtitlesStale ? <p role="status" className="text-body-sm text-warning">Los subtítulos han cambiado desde que creaste el resumen. Rehazlo para aplicarlos.</p> : null}
          </>
        ) : null}

        {summary !== null && !done && !running ? (
          <div className="grid aspect-video w-full place-items-center rounded bg-surface-2 px-6 text-center text-body-sm text-fg-3">
            Elige las jugadas y crea tu resumen. Aparecerá aquí.
          </div>
        ) : null}

        {running ? (
          <div className="flex flex-col gap-2">
            <p className="text-body-sm text-fg-2" aria-live="polite">
              Creando resumen · {Math.round((summary.progress ?? 0) * 100)} %
            </p>
            <Progress tone="stream" size="sm" value={(summary.progress ?? 0) * 100} />
            <p className="text-body-sm text-fg-3">Puedes salir de esta pantalla: el resumen sigue creándose en este PC.</p>
          </div>
        ) : null}

        {summary?.status === 'failed' || requestError ? (
          <p role="alert" className="text-body-sm text-destructive">
            {requestError ?? summary?.error ?? FAIL_MESSAGE}
          </p>
        ) : null}
      </section>

      <section aria-label="Jugadas del resumen" className="flex min-w-0 flex-col gap-3 rounded-md border border-border-subtle bg-surface-1 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="font-semibold">
            Jugadas del resumen <span className="text-fg-3 tabular-nums">{selectedPlays.length} de {plays.length}</span>
          </h2>
          {hasKills ? (
            <Button variant="ghost" size="sm" disabled={running} onClick={() => setChosen(bulk.next)}>
              {bulk.label}
            </Button>
          ) : null}
        </div>
        <ul className="flex max-h-[28rem] flex-col overflow-y-auto">
          {ordered.map((play) => (
            <li key={play.id}>
              <label className="flex min-h-11 cursor-pointer items-center gap-3 text-body-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-primary"
                  checked={selected.has(play.id)}
                  disabled={running}
                  onChange={() => toggle(play.id)}
                />
                <span className="min-w-0 flex-1 truncate font-semibold">{play.label}</span>
                {isVoiceOnly(play) ? <Mic aria-label="Pico de voz" className="size-3.5 shrink-0 text-fg-3" /> : null}
                <span className="shrink-0 text-fg-3 tabular-nums">{playTimeLabel(play)}</span>
              </label>
            </li>
          ))}
        </ul>
        {summary !== null && !running && (summary.status !== 'done' || stale) ? (
          <>
            <p className="text-body-sm text-fg-2">{summaryChoiceText(selectedPlays)}</p>
            <Button variant="stream" disabled={!canStart} onClick={() => void start()}>
              {startLabel(summary.status, stale)}
            </Button>
          </>
        ) : null}
        <details className="rounded-md border border-border-subtle p-3">
          <DisclosureSummary className="font-semibold text-body-sm">Subtítulos · opcional</DisclosureSummary>
          <div className="mt-3">
            <StreamSubtitlesPanel jobId={jobId} kind="summary" video={{ id: SUMMARY_VIDEO, windows: stretches }} stretches={stretches} disabled={running}
              renderKey={summary?.rendered_at ?? ''} onStale={setSubtitlesStale} />
          </div>
        </details>
      </section>
    </div>
  );
}
