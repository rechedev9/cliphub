'use client';

import { useRef, useState, type DragEvent, type ReactNode } from 'react';
import { UploadCloud } from 'lucide-react';
import { isStreamURLValidationError } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
      className="studio-panel studio-panel-raised flex max-w-[1080px] flex-col gap-2.5 border-stream/45 px-4.5 py-3.5 shadow-[var(--elev-3),var(--glow-stream-md)]"
    >
      <div className="flex justify-between gap-3 font-mono text-meta uppercase tracking-widest text-fg-3">
        <span id="stream-source-title" className="text-stream-text">
          Nueva fuente
        </span>
        <span>Salida 9:16 · 1080p</span>
      </div>

      <form
        className="grid items-center gap-3 @[44rem]/content:grid-cols-[minmax(0,1fr)_220px_auto]"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmitUrl();
        }}
      >
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
        <Input
          id="stream-title"
          aria-label="Título (opcional)"
          placeholder="Título (opcional)"
          value={title}
          disabled={submitting}
          onChange={(e) => onTitleChange(e.target.value)}
          className="font-mono"
        />
        <Button
          type="submit"
          variant="stream"
          className="neon-notch h-11 font-display uppercase tracking-wide"
          disabled={submitting}
          loading={submitting}
          loadingText="Trayendo clip…"
        >
          Traer clip
        </Button>
      </form>
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
          'hover:border-stream hover:bg-stream/10 hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50',
          dragging ? 'border-stream bg-stream/10 text-fg-1' : 'border-stream/45',
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
