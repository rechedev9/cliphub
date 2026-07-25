'use client';

import { useRef, type ReactNode } from 'react';
import { Film, Link2, MonitorPlay, ShieldCheck, Sparkles, Twitch, UploadCloud } from 'lucide-react';
import type { StreamJob } from '@/lib/api/streams';
import { isStreamURLValidationError } from '@/lib/streams/plan';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { StreamOutputAside } from '@/components/streams/output-aside';

const OUTPUT_NOTES = [
  { icon: Twitch, text: 'Twitch y YouTube compatibles', tone: 'stream' as const },
  { icon: ShieldCheck, text: 'Procesado en este PC', tone: 'success' as const },
];

/**
 * Stage 1: where a Twitch/YouTube URL or a local MP4 becomes a stream job, plus
 * the recoverable drafts from previous sessions.
 *
 * The `#stream-url` id, the `aria-invalid` flag it raises for a rejected URL,
 * the "TRAER CLIP" label and the exact "CONTINUAR BORRADOR" draft text are read
 * by the packaged release E2E; they are contract, not styling.
 */
export function StreamSourceCard({
  sourceUrl,
  title,
  submitting,
  error,
  recoverableJobs,
  onSourceUrlChange,
  onTitleChange,
  onSubmitUrl,
  onSubmitFile,
  onResume,
}: {
  sourceUrl: string;
  title: string;
  submitting: boolean;
  error: string | null;
  recoverableJobs: StreamJob[];
  onSourceUrlChange: (v: string) => void;
  onTitleChange: (v: string) => void;
  onSubmitUrl: () => void;
  onSubmitFile: (file: File) => void;
  onResume: (job: StreamJob) => void;
}): ReactNode {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const urlError = isStreamURLValidationError(error) ? error : null;

  return (
    <div className="@container/source studio-panel studio-panel-raised relative max-w-5xl p-5 sm:p-7">
      <div className="grid gap-8 @[46rem]/source:grid-cols-[minmax(0,1fr)_16.5rem] @[46rem]/source:items-stretch">
        <div className="min-w-0">
          <SectionEyebrow label="FUENTE" accent="magenta" />
          <p className="mt-3 max-w-xl text-body text-fg-2">
            Importa el vídeo completo. En el siguiente paso eliges encuadre, rangos, música y efectos.
          </p>

          <div className="mt-6 flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="stream-title" className="text-label text-fg-2">
                Título (opcional)
              </Label>
              <Input
                id="stream-title"
                placeholder="Clutch 1v5 en pistola"
                value={title}
                disabled={submitting}
                onChange={(e) => onTitleChange(e.target.value)}
              />
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="stream-url" className="text-label text-fg-2">
                URL de clip o VOD de Twitch o YouTube
              </Label>
              <div className="flex flex-col gap-3 @[30rem]/source:flex-row">
                <div className="relative flex-1">
                  <Link2 className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-fg-3" />
                  <Input
                    id="stream-url"
                    placeholder="https://clips.twitch.tv/…"
                    value={sourceUrl}
                    disabled={submitting}
                    aria-invalid={urlError !== null || undefined}
                    aria-describedby={urlError ? 'stream-url-error' : undefined}
                    onChange={(e) => onSourceUrlChange(e.target.value)}
                    className="pl-10"
                  />
                </div>
                <Button type="button" variant="stream" onClick={onSubmitUrl} disabled={submitting} loading={submitting}>
                  {submitting ? null : <Sparkles className="size-4" />}
                  TRAER CLIP
                </Button>
              </div>
              {urlError ? (
                <p id="stream-url-error" role="alert" className="text-body-sm text-destructive">
                  {urlError}
                </p>
              ) : null}
            </div>

            <div className="flex items-center gap-3.5 font-mono text-meta uppercase tracking-wider text-fg-3">
              <span aria-hidden className="h-px flex-1 bg-border" />
              O USA UN ARCHIVO LOCAL
              <span aria-hidden className="h-px flex-1 bg-border" />
            </div>

            <Button
              type="button"
              variant="outline"
              disabled={submitting}
              onClick={() => fileInputRef.current?.click()}
              className="w-full border-dashed border-stream/45 bg-surface-2 hover:border-stream hover:bg-stream/10"
            >
              <UploadCloud className="size-4" />
              SUBIR UN MP4
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept="video/mp4,.mp4"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0];
                e.target.value = '';
                if (file) onSubmitFile(file);
              }}
            />

            {error && !urlError ? (
              <p role="alert" className="text-body-sm text-destructive">
                {error}
              </p>
            ) : null}

            {recoverableJobs.length > 0 ? (
              <section className="flex flex-col gap-2 border-t border-border pt-5" aria-labelledby="stream-drafts-title">
                <h3 id="stream-drafts-title" className="font-display text-label font-bold uppercase tracking-wide text-fg-1">
                  Borradores recientes
                </h3>
                {recoverableJobs.map((candidate) => (
                  <button
                    key={candidate.id}
                    type="button"
                    disabled={submitting}
                    onClick={() => onResume(candidate)}
                    className="flex min-h-11 items-center justify-between gap-3 border border-border bg-surface-2 px-3.5 py-2 text-left transition-colors duration-(--dur-fast) ease-standard hover:border-stream/60 hover:bg-stream/[0.07] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50"
                  >
                    <span className="flex min-w-0 items-center gap-2.5">
                      <Film aria-hidden className="size-4 shrink-0 text-fg-3" />
                      <span className="min-w-0 truncate text-body-sm text-fg-1">
                        {candidate.title?.trim() || 'Clip sin título'}
                      </span>
                    </span>
                    <StatusTag tone="stream">CONTINUAR BORRADOR</StatusTag>
                  </button>
                ))}
              </section>
            ) : null}
          </div>
        </div>

        <StreamOutputAside heading="Salida" spec="9:16 · 1080p" icon={MonitorPlay} notes={OUTPUT_NOTES} />
      </div>
    </div>
  );
}
