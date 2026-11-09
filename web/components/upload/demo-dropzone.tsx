'use client';

import { useCallback, useEffect, useId, useRef, useState, type DragEvent, type ReactNode } from 'react';
import { AlertTriangle, ArrowUp, FileUp } from 'lucide-react';
import { MAX_DEMO_FILES } from '@/lib/upload/demo-names';
import { expandDemoUploads } from '@/lib/upload/expand-archives';
import { cn } from '@/lib/utils';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { buttonVariants } from '@/components/ui/button';

export type DemoDropzoneProps = {
  /** Called with the chosen .dem file(s). The parent owns parsing + navigation. */
  onFiles: (files: File[]) => void;
  compact?: boolean;
  disabled?: boolean;
  /** Tailwind min-height for the full layout; the flow homes set 200px. */
  minHeightClass?: string;
  /** Draws the Crear tile in this flow's colour instead of the plain drop zone. */
  tile?: DropzoneTile;
};

export type DropzoneTile = {
  tone: FormatTone;
  title: string;
  /** The first demo of the flow: a tinted ground and a filled icon. */
  strong?: boolean;
  /** Extra buttons beside "Elegir archivo", outside the label: they keep working while a file extracts. */
  actions?: ReactNode;
};

const DROPZONE_TITLE = 'Arrastra tu demo o elige un archivo';

/** Drop zone for .dem / archives; the label opens the native file dialog. */
export function DemoDropzone({
  onFiles,
  compact = false,
  disabled = false,
  minHeightClass = 'min-h-[260px]',
  tile,
}: DemoDropzoneProps): ReactNode {
  const inputId = useId();
  const [interactive, setInteractive] = useState(false);
  useEffect(() => setInteractive(true), []);
  const [dragging, setDragging] = useState(false);
  const dragDepth = useRef(0);
  const [extracting, setExtracting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const busy = !interactive || extracting || disabled;

  const resetDrag = useCallback(() => {
    dragDepth.current = 0;
    setDragging(false);
  }, []);

  useEffect(() => {
    if (busy) resetDrag();
  }, [busy, resetDrag]);

  useEffect(() => {
    window.addEventListener('drop', resetDrag);
    window.addEventListener('dragend', resetDrag);
    return () => {
      window.removeEventListener('drop', resetDrag);
      window.removeEventListener('dragend', resetDrag);
    };
  }, [resetDrag]);

  const accept = useCallback(
    (fileList: FileList | null | undefined) => {
      const files = fileList ? Array.from(fileList) : [];
      if (files.length === 0 || busy) return;
      setError(null);
      setExtracting(true);
      void expandDemoUploads(files)
        .then((result) => {
          if (!result.ok) {
            setError(result.error);
            return;
          }
          onFiles(result.files);
        })
        .catch(() => setError('No se pudo leer el archivo. Comprueba que la descarga esté completa y vuelve a elegirlo.'))
        .finally(() => setExtracting(false));
    },
    [busy, onFiles],
  );

  const dropHandlers = {
    onDragEnter: (e: DragEvent<HTMLElement>) => {
      e.preventDefault();
      if (busy) return;
      dragDepth.current += 1;
      setDragging(true);
    },
    onDragOver: (e: DragEvent<HTMLElement>) => e.preventDefault(),
    onDragLeave: () => {
      dragDepth.current = Math.max(0, dragDepth.current - 1);
      if (dragDepth.current === 0) setDragging(false);
    },
    onDrop: (e: DragEvent<HTMLElement>) => {
      e.preventDefault();
      resetDrag();
      if (!busy) accept(e.dataTransfer.files);
    },
  };

  const fileInput = (
    <input
      id={inputId}
      type="file"
      aria-label="Elegir demos de CS2"
      multiple
      accept=".dem,.zst,.rar,.zip"
      disabled={busy}
      className="sr-only"
      onClick={(e) => {
        e.currentTarget.value = '';
      }}
      onChange={(e) => accept(e.target.files)}
    />
  );

  const errorLine = error ? (
    <p
      role="alert"
      className="studio-shake flex items-start gap-2.5 border border-destructive bg-surface-2 px-4 py-3 text-body-sm text-destructive"
    >
      <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
      {error}
    </p>
  ) : null;

  if (tile) {
    // The label stretches over the tile and the actions sit above it, so they keep their own name and clicks.
    return (
      <div className="flex flex-col gap-3">
        <div
          data-slot="dropzone"
          data-dragging={dragging ? 'true' : undefined}
          data-layout="full"
          {...dropHandlers}
          aria-busy={busy || undefined}
          className={tileClass({ tile, dragging, busy, minHeightClass })}
        >
          <label htmlFor={inputId} aria-hidden className={cn('absolute inset-0 cursor-pointer rounded-2xl', busy && 'pointer-events-none cursor-wait')} />
          <TileContent tile={tile} extracting={extracting} />
          {fileInput}
        </div>
        {errorLine}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <label
        htmlFor={inputId}
        data-slot="dropzone"
        data-dragging={dragging ? 'true' : undefined}
        data-layout={compact ? 'compact' : 'full'}
        {...dropHandlers}
        aria-busy={busy || undefined}
        aria-disabled={busy || undefined}
        className={cn(
          'group flex cursor-pointer items-center justify-center rounded-lg border border-dashed text-center',
          'transition-[background-color,border-color] duration-(--dur-base) ease-standard',
          compact ? 'min-h-24 gap-4 px-4 py-4' : cn('flex-col gap-3.5 px-6 py-8', minHeightClass),
          busy && 'pointer-events-none cursor-wait',
          'has-[:focus-visible]:border-primary has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
          dragging
            ? 'border-primary bg-surface-3'
            : 'border-border-accent bg-surface-1 hover:border-primary hover:bg-surface-2',
        )}
      >
        <span
          aria-hidden
          className={cn(
            'grid shrink-0 place-items-center border border-border-accent bg-surface-0 text-fg-2',
            compact ? 'size-10' : 'size-14',
          )}
        >
          <ArrowUp className={compact ? 'size-5' : 'size-7'} strokeWidth={1.7} />
        </span>
        <span className={cn('flex min-w-0 flex-col', compact ? 'items-start gap-1 text-left' : 'items-center gap-3.5')}>
          <span className={cn('font-display font-semibold text-fg-1', compact ? 'text-body' : 'text-title')}>
            {extracting ? 'Extrayendo archivo…' : DROPZONE_TITLE}
          </span>
          {!compact && !extracting ? <span className="rounded-md border border-border-strong bg-surface-3 px-4 py-2 text-body-sm font-semibold text-fg-1">Elegir archivo</span> : null}
          {compact ? (
            <span className="text-body-sm text-fg-3">.dem, .zst, .rar o .zip</span>
          ) : (
            <span className="text-meta text-fg-3">
              .dem · .dem.zst · .rar · .zip · hasta {MAX_DEMO_FILES} demos
            </span>
          )}
        </span>
        {fileInput}
      </label>
      {errorLine}
    </div>
  );
}

function tileClass({ tile, dragging, busy, minHeightClass }: { tile: DropzoneTile; dragging: boolean; busy: boolean; minHeightClass: string }): string {
  const tone = FORMAT_TONE[tile.tone];
  let ground = 'border-border-strong hover:border-fg-4 hover:bg-surface-2';
  if (tile.strong) ground = cn(tone.border, tone.tint, 'hover:bg-surface-2');
  return cn(
    'group relative flex flex-1 flex-col items-center justify-center gap-2.5 rounded-2xl border-[1.5px] border-dashed p-5 text-center',
    'transition-[background-color,border-color] duration-(--dur-base) ease-standard',
    minHeightClass,
    busy && 'cursor-wait',
    'has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2 has-[input:focus-visible]:outline-ring',
    dragging ? 'border-fg-2 bg-surface-3' : ground,
  );
}

function TileContent({ tile, extracting }: { tile: DropzoneTile; extracting: boolean }): ReactNode {
  const tone = FORMAT_TONE[tile.tone];
  return (
    <>
      <span
        aria-hidden
        className={cn('grid shrink-0 place-items-center', tile.strong ? cn('mb-1 size-16 rounded-2xl', tone.fill) : cn('size-12 rounded-xl text-fg-1', tone.tint))}
      >
        <FileUp className={tile.strong ? 'size-7' : 'size-6'} strokeWidth={1.8} />
      </span>
      <span className={cn('font-display font-extrabold text-fg-1', tile.strong ? 'text-section' : 'text-body')}>
        {extracting ? 'Extrayendo archivo…' : tile.title}
      </span>
      <span className={cn('text-fg-3', tile.strong ? 'text-body-sm' : 'text-meta')}>
        .dem, .dem.zst, .zip o .rar · hasta {MAX_DEMO_FILES} demos{tile.strong ? '. Se analiza en este PC.' : ''}
      </span>
      {extracting && tile.actions === undefined ? null : (
        <span className="pointer-events-none mt-1 flex flex-wrap justify-center gap-2">
          {extracting ? null : <span className={buttonVariants({ variant: tile.strong ? 'default' : 'outline', size: 'sm' })}>Elegir archivo</span>}
          {tile.actions ? <span className="pointer-events-auto relative z-10 flex flex-wrap justify-center gap-2">{tile.actions}</span> : null}
        </span>
      )}
    </>
  );
}
