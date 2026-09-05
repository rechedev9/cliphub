'use client';

import { useRef, useState, type DragEvent, type ReactNode } from 'react';
import { ArrowRight, Info, Link2, Twitch, UploadCloud, Youtube } from 'lucide-react';
import { isStreamURLValidationError } from '@/lib/streams/plan';
import { Button, buttonVariants, FOCUS_RING } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

/** Both source choices share the optional title. Keep #stream-url stable for integrations. */
export function StreamSourcePanel({
  sourceUrl,
  title,
  submitting,
  error,
  onSourceUrlChange,
  onTitleChange,
  onSubmitUrl,
  onSubmitFile,
}: {
  sourceUrl: string;
  title: string;
  submitting: boolean;
  error: string | null;
  onSourceUrlChange: (value: string) => void;
  onTitleChange: (value: string) => void;
  onSubmitUrl: () => void;
  onSubmitFile: (file: File) => void;
}): ReactNode {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const urlError = isStreamURLValidationError(error) ? error : null;

  const handleDrop = (event: DragEvent<HTMLButtonElement>): void => {
    event.preventDefault();
    setDragging(false);
    if (submitting) return;
    const file = event.dataTransfer.files[0];
    if (file) onSubmitFile(file);
  };

  return (
    <section
      aria-labelledby="stream-source-title"
      className="studio-panel flex flex-col gap-6 bg-surface-3 bg-none p-5 @[40rem]/content:p-6"
    >
      <div>
        <h2 id="stream-source-title" className="font-display text-title font-semibold text-fg-1">Importa tu vídeo</h2>
        <p className="mt-1.5 text-body text-fg-2">Pega un enlace o sube una grabación para empezar.</p>
      </div>

      <div className="grid gap-6 @[48rem]/content:grid-cols-[minmax(0,1.1fr)_auto_minmax(0,1fr)] @[48rem]/content:gap-8">
        <form
          className="flex min-w-0 flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!submitting) onSubmitUrl();
          }}
        >
          <ul aria-label="Plataformas compatibles" className="flex flex-wrap items-center gap-x-7 gap-y-3 pb-1 text-body text-fg-1">
            <li className="flex items-center gap-2"><Twitch aria-hidden className="size-5 text-chart-3" />Twitch</li>
            <li className="flex items-center gap-2"><Youtube aria-hidden className="size-6 text-destructive" />YouTube</li>
            <li className="flex items-center gap-2"><span aria-hidden className="font-display text-section font-bold leading-none text-success">K</span>Kick</li>
          </ul>
          <div className="flex min-w-0 flex-col gap-2">
            <Label htmlFor="stream-url">Enlace del vídeo</Label>
            <div className="relative">
              <Link2 aria-hidden className="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-fg-3" />
              <Input
                id="stream-url"
                aria-label="Enlace del vídeo de Twitch, YouTube o Kick"
                placeholder="Pega un enlace de Twitch, YouTube o Kick"
                autoComplete="off"
                spellCheck={false}
                inputMode="url"
                value={sourceUrl}
                disabled={submitting}
                aria-invalid={urlError !== null || undefined}
                aria-describedby={urlError ? 'stream-url-error stream-source-hint' : 'stream-source-hint'}
                onChange={(e) => onSourceUrlChange(e.target.value)}
                className="h-12 bg-surface-2 pl-11"
              />
            </div>
            {urlError ? (
              <p id="stream-url-error" role="alert" className="text-body-sm text-destructive">{urlError}</p>
            ) : null}
          </div>
          <div className="flex min-w-0 flex-col gap-2">
            <div className="flex items-center justify-between gap-2">
              <Label htmlFor="stream-title">Nombre del proyecto</Label>
              <span id="stream-title-hint" className="text-body-sm text-fg-3">Opcional</span>
            </div>
            <Input
              id="stream-title"
              aria-describedby="stream-title-hint"
              placeholder="Ej. Clutch 1v4"
              value={title}
              disabled={submitting}
              onChange={(e) => onTitleChange(e.target.value)}
              className="h-12 bg-surface-2"
            />
            <Button
              type="submit"
              variant="stream"
              size="lg"
              className="mt-1 w-full shadow-sm hover:shadow-md"
              disabled={submitting}
              loading={submitting}
              loadingText="Importando…"
            >
              Importar vídeo<ArrowRight aria-hidden className="size-5" />
            </Button>
          </div>
        </form>

        <div aria-hidden className="flex items-center gap-3 text-body-sm text-fg-3 @[48rem]/content:flex-col">
          <span className="h-px flex-1 bg-border @[48rem]/content:h-auto @[48rem]/content:w-px" />
          <span>o</span>
          <span className="h-px flex-1 bg-border @[48rem]/content:h-auto @[48rem]/content:w-px" />
        </div>

        <button
          type="button"
          aria-label="Seleccionar archivo MP4"
          aria-describedby="stream-source-hint"
          disabled={submitting}
          onClick={() => fileInputRef.current?.click()}
          onDragOver={(event) => {
            event.preventDefault();
            if (!submitting) setDragging(true);
          }}
          onDragLeave={(event) => {
            if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) setDragging(false);
          }}
          onDrop={handleDrop}
          className={cn(
            'group/upload flex min-h-60 w-full flex-col items-center justify-center rounded-lg border border-dashed bg-surface-2 px-5 py-8 text-center transition-colors duration-(--dur-fast) ease-standard',
            'hover:border-primary hover:bg-surface-4 disabled:pointer-events-none disabled:opacity-50',
            FOCUS_RING,
            dragging ? 'border-primary bg-surface-4' : 'border-border-strong',
          )}
        >
          <UploadCloud aria-hidden className="mb-5 size-10 text-fg-3 group-hover/upload:text-primary" strokeWidth={1.5} />
          <span className="font-display text-body-lg font-semibold text-fg-1">{dragging ? 'Suelta tu vídeo aquí' : 'Arrastra tu vídeo aquí'}</span>
          <span className="mt-1.5 text-body-sm text-fg-2">Archivo MP4</span>
          <span className={buttonVariants({ variant: 'outline', className: 'mt-5 bg-surface-2' })}>Seleccionar archivo</span>
        </button>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept="video/mp4,.mp4"
        disabled={submitting}
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          e.target.value = '';
          if (file) onSubmitFile(file);
        }}
      />

      <p id="stream-source-hint" className="flex items-start gap-2.5 border-t border-border pt-4 text-body-sm text-fg-2">
        <Info aria-hidden className="mt-0.5 size-4 shrink-0" />
        <span>Después podrás elegir los cortes. Cada corte se exporta como un Short independiente.</span>
      </p>

      {error && !urlError ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
    </section>
  );
}
