'use client';

import { useRef, useState, type DragEvent, type ReactNode } from 'react';
import { UploadCloud } from 'lucide-react';
import { isStreamURLValidationError } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

/** NUEVA FUENTE: paste a URL or drop an MP4. E2E reads #stream-url. */
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
      className="studio-panel studio-panel-raised measure-list flex flex-col gap-2.5 px-4.5 py-3.5"
    >
      <h2 id="stream-source-title" className="font-mono text-meta uppercase tracking-widest text-fg-3">
        Importar vídeo de origen
      </h2>

      <form
        className="grid items-end gap-3 @[44rem]/content:grid-cols-[minmax(0,1fr)_220px_auto]"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmitUrl();
        }}
      >
        <div className="flex min-w-0 flex-col gap-2">
          <Label htmlFor="stream-url">Enlace de Twitch, YouTube o Kick</Label>
          <Input
            id="stream-url"
            aria-label="URL de clip o VOD de Twitch, YouTube o Kick"
            placeholder="https://kick.com/canal/clips/…"
            value={sourceUrl}
            disabled={submitting}
            aria-invalid={urlError !== null || undefined}
            aria-describedby={urlError ? 'stream-url-error' : undefined}
            onChange={(e) => onSourceUrlChange(e.target.value)}
            className="font-mono"
          />
        </div>
        <div className="flex min-w-0 flex-col gap-2">
          <Label htmlFor="stream-title">Nombre del proyecto (opcional)</Label>
          <Input
            id="stream-title"
            aria-label="Título (opcional)"
            placeholder="Título (opcional)"
            value={title}
            disabled={submitting}
            onChange={(e) => onTitleChange(e.target.value)}
            className="font-mono"
          />
        </div>
        <Button
          type="submit"
          variant="stream"
          className="neon-notch h-11 font-display uppercase tracking-wide"
          disabled={submitting}
          loading={submitting}
          loadingText="Importando…"
        >
          Importar vídeo
        </Button>
      </form>
      <p className="text-body-sm text-fg-3">Después elegirás los fragmentos. Cada corte se exporta como un Short independiente.</p>
      {urlError ? (
        <p id="stream-url-error" role="alert" className="text-body-sm text-destructive">
          {urlError}
        </p>
      ) : null}

      <div className="flex items-center gap-3.5 font-mono text-meta uppercase tracking-wider text-fg-3">
        <span aria-hidden className="h-px flex-1 bg-border" />
        o usa un archivo local
        <span aria-hidden className="h-px flex-1 bg-border" />
      </div>

      <button
        type="button"
        disabled={submitting}
        onClick={() => fileInputRef.current?.click()}
        onDragOver={(event) => {
          event.preventDefault();
          if (!submitting) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        className={cn(
          'flex min-h-11 w-full items-center justify-center gap-2 border border-dashed bg-surface-2 font-display text-body-sm font-semibold uppercase tracking-wide text-fg-2 transition-colors duration-(--dur-fast) ease-standard',
          // surface-4, not -3: the panel around this well is already -3, so a
          // hover into -3 flattens the control into its own parent.
          'hover:border-primary/60 hover:bg-surface-4 hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50',
          dragging ? 'border-primary bg-surface-4 text-fg-1' : 'border-border-strong',
        )}
      >
        <UploadCloud aria-hidden className="size-4" />
        Subir un MP4 · arrástralo aquí
      </button>
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
    </section>
  );
}
