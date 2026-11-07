'use client';

import { useRef, useState, type DragEvent, type ReactNode } from 'react';
import { ArrowRight, Info, Link2, UploadCloud } from 'lucide-react';
import type { StreamSourceKind } from '@/lib/api/streams';
import { formatUploadBytes, uploadSent, type UploadProgress } from '@/lib/api/upload';
import { isStreamURLValidationError } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { LongOperation } from '@/components/studio/long-operation';
import { cn } from '@/lib/utils';

/** A failed import, tagged with the control that caused it so the message sits beside it. */
export type StreamImportError = { source: 'url' | 'file'; message: string };

const KIND_COPY: Record<StreamSourceKind, { placeholder: string; result: string }> = {
  clip: {
    placeholder: 'Pega el enlace de un clip de Twitch, YouTube o Kick',
    result: 'Un momento corto: lo pasas a vertical, ajustas la cámara y lo exportas como un Short.',
  },
  vod: {
    placeholder: 'Pega el enlace del VOD de Twitch, YouTube o Kick',
    result: 'Un directo entero: detectamos tus kills y sacas un resumen horizontal de las que elijas y Shorts verticales.',
  },
};

/** The import hero. Both source choices share the optional title. Keep #stream-url stable for integrations. */
export function StreamSourcePanel({
  sourceKind,
  sourceUrl,
  title,
  submitting,
  upload = null,
  error,
  onSourceKindChange,
  onSourceUrlChange,
  onTitleChange,
  onSubmitUrl,
  onSubmitFile,
}: {
  sourceKind: StreamSourceKind;
  sourceUrl: string;
  title: string;
  submitting: boolean;
  /** The MP4 being sent; null when no file import is in flight. */
  upload?: UploadProgress | null;
  error: StreamImportError | null;
  onSourceKindChange: (kind: StreamSourceKind) => void;
  onSourceUrlChange: (value: string) => void;
  onTitleChange: (value: string) => void;
  onSubmitUrl: () => void;
  onSubmitFile: (file: File) => void;
}): ReactNode {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const urlError = error !== null && isStreamURLValidationError(error.message) ? error.message : null;
  const importError = error !== null && urlError === null ? error : null;

  const handleDrop = (event: DragEvent<HTMLElement>): void => {
    event.preventDefault();
    setDragging(false);
    if (submitting) return;
    const file = event.dataTransfer.files[0];
    if (file) onSubmitFile(file);
  };

  return (
    // The whole band takes a dropped MP4, so the drop target is not a small button.
    <section
      aria-labelledby="stream-source-title"
      onDragOver={(event) => {
        event.preventDefault();
        if (!submitting) setDragging(true);
      }}
      onDragLeave={(event) => {
        if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) setDragging(false);
      }}
      onDrop={handleDrop}
      className={cn(
        'flex flex-col gap-4.5 rounded-3xl border p-5 transition-colors duration-(--dur-fast) ease-standard @[40rem]/content:p-7',
        FORMAT_TONE.stream.tint,
        dragging ? 'border-dashed border-format-stream' : FORMAT_TONE.stream.border,
      )}
    >
      <div className="flex flex-wrap items-end gap-4.5">
        <div className="flex min-w-0 flex-[1_1_26rem] flex-col gap-2">
          <h1 id="stream-source-title" className="font-display text-[2.5rem] leading-[1.05] font-extrabold tracking-[-0.025em] text-fg-1">
            De tu directo a Shorts
          </h1>
          <p className="measure-read text-body text-fg-2">
            Pega el enlace de un VOD o de un clip, o sube tu grabación. Cada momento que elijas sale como un Short vertical.
          </p>
        </div>
        <span id="stream-kind-label" className="sr-only">Qué vas a importar</span>
        <ToggleGroup
          type="single"
          variant="filter"
          spacing={2}
          aria-labelledby="stream-kind-label"
          aria-describedby="stream-source-hint"
          disabled={submitting}
          value={sourceKind}
          onValueChange={(next: string) => {
            if (next === 'clip' || next === 'vod') onSourceKindChange(next);
          }}
        >
          <ToggleGroupItem value="vod">VOD completo</ToggleGroupItem>
          <ToggleGroupItem value="clip">Clip</ToggleGroupItem>
        </ToggleGroup>
      </div>

      <form
        className="flex min-w-0 flex-col gap-3"
        onSubmit={(event) => {
          event.preventDefault();
          if (!submitting) onSubmitUrl();
        }}
      >
        <Label htmlFor="stream-url" className="sr-only">Enlace del vídeo</Label>
        <div
          className={cn(
            'flex min-w-0 flex-wrap items-center gap-2.5 rounded-2xl border-2 bg-surface-0 py-2 pr-2 pl-4.5',
            urlError ? 'border-destructive' : 'border-format-stream/70',
          )}
        >
          <Link2 aria-hidden className={cn('size-5 shrink-0', FORMAT_TONE.stream.text)} />
          <Input
            id="stream-url"
            aria-label="Enlace del vídeo de Twitch, YouTube o Kick"
            placeholder={KIND_COPY[sourceKind].placeholder}
            autoComplete="off"
            spellCheck={false}
            inputMode="url"
            value={sourceUrl}
            disabled={submitting}
            aria-invalid={urlError !== null || undefined}
            aria-describedby={urlError ? 'stream-url-error stream-source-hint' : 'stream-source-hint'}
            onChange={(e) => onSourceUrlChange(e.target.value)}
            className="h-11 min-w-48 flex-1 border-0 bg-transparent px-1 text-body-lg shadow-none focus-visible:bg-transparent"
          />
          <Button
            type="submit"
            variant="stream"
            className="rounded-xl px-5 font-extrabold"
            disabled={submitting}
            loading={submitting}
            loadingText="Importando…"
          >
            Importar<ArrowRight aria-hidden />
          </Button>
        </div>
        {urlError ? (
          <p id="stream-url-error" role="alert" className="text-body-sm text-destructive">{urlError}</p>
        ) : null}
        {importError?.source === 'url' ? (
          <p role="alert" className="text-body-sm text-destructive">{importError.message}</p>
        ) : null}
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1.5">
          <Label htmlFor="stream-title" className="text-label text-fg-2">Nombre del proyecto</Label>
          <Input
            id="stream-title"
            aria-describedby="stream-title-hint"
            placeholder="Ej. Clutch 1v4"
            value={title}
            disabled={submitting}
            onChange={(e) => onTitleChange(e.target.value)}
            className="h-9 max-w-72 min-w-40 flex-1 bg-surface-1"
          />
          <span id="stream-title-hint" className="text-body-sm text-fg-3">Opcional</span>
        </div>
      </form>

      <div className="flex flex-wrap items-center justify-end gap-x-4.5 gap-y-3">
        <span className="text-body-sm text-fg-2">{dragging ? 'Suelta tu vídeo aquí' : '¿Lo tienes en el PC?'}</span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label="Seleccionar archivo MP4"
          aria-describedby="stream-source-hint"
          disabled={submitting}
          onClick={() => fileInputRef.current?.click()}
          className="bg-surface-1"
        >
          <UploadCloud aria-hidden />Subir un MP4
        </Button>
      </div>
      {upload ? <FileUploadProgress upload={upload} /> : null}
      {importError?.source === 'file' ? (
        <p role="alert" className="text-body-sm text-destructive">{importError.message}</p>
      ) : null}

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

      <p id="stream-source-hint" className="flex items-start gap-2.5 border-t border-format-stream/25 pt-4 text-body-sm text-fg-2">
        <Info aria-hidden className="mt-0.5 size-4 shrink-0" />
        <span>{KIND_COPY[sourceKind].result} También puedes arrastrar un MP4 hasta aquí.</span>
      </p>
    </section>
  );
}

/** Bytes first, then the server-side import once every byte has left. */
function FileUploadProgress({ upload }: { upload: UploadProgress }): ReactNode {
  if (uploadSent(upload)) return <LongOperation stage="Importando…" tone="stream" />;
  return <LongOperation stage="Subiendo" percent={upload.percent} detail={formatUploadBytes(upload)} tone="stream" />;
}
