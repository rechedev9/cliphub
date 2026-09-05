'use client';

import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, ArrowUp } from 'lucide-react';
import { MAX_DEMO_FILES } from '@/lib/upload/demo-names';
import { expandDemoUploads } from '@/lib/upload/expand-archives';
import { cn } from '@/lib/utils';

export type DemoDropzoneProps = {
  /** Called with the chosen .dem file(s). The parent owns parsing + navigation. */
  onFiles: (files: File[]) => void;
  compact?: boolean;
  disabled?: boolean;
  /** Tailwind min-height for the full layout; the spec sets 240px on the hub and 260px on /clips/nueva. */
  minHeightClass?: string;
};

const DROPZONE_TITLE = 'Arrastra tu demo o elige un archivo';

/** Drop zone for .dem / archives; the label opens the native file dialog. */
export function DemoDropzone({
  onFiles,
  compact = false,
  disabled = false,
  minHeightClass = 'min-h-[260px]',
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

  return (
    <div className="flex flex-col gap-3">
      <label
        htmlFor={inputId}
        data-slot="dropzone"
        data-dragging={dragging ? 'true' : undefined}
        data-layout={compact ? 'compact' : 'full'}
        onDragEnter={(e) => {
          e.preventDefault();
          if (busy) return;
          dragDepth.current += 1;
          setDragging(true);
        }}
        onDragOver={(e) => e.preventDefault()}
        onDragLeave={() => {
          dragDepth.current = Math.max(0, dragDepth.current - 1);
          if (dragDepth.current === 0) setDragging(false);
        }}
        onDrop={(e) => {
          e.preventDefault();
          resetDrag();
          if (!busy) accept(e.dataTransfer.files);
        }}
        aria-busy={busy || undefined}
        aria-disabled={busy || undefined}
        className={cn(
          'group flex cursor-pointer items-center justify-center rounded-[10px] border border-dashed text-center',
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
            'grid shrink-0 place-items-center border border-border-accent bg-surface-0 text-primary',
            compact ? 'size-10' : 'size-14',
          )}
        >
          <ArrowUp className={compact ? 'size-5' : 'size-7'} strokeWidth={1.7} />
        </span>
        <span className={cn('flex min-w-0 flex-col', compact ? 'items-start gap-1 text-left' : 'items-center gap-3.5')}>
          <span className={cn('font-display font-semibold text-fg-1', compact ? 'text-body' : 'text-title')}>
            {extracting ? 'Extrayendo archivo…' : DROPZONE_TITLE}
          </span>
          {!compact && !extracting ? <span className="rounded-md border border-border-strong bg-surface-3 px-4 py-2 text-body-sm font-semibold text-primary">Elegir archivo</span> : null}
          <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
            .dem · .dem.zst · .rar · .zip · hasta {MAX_DEMO_FILES} demos
          </span>
        </span>

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
      </label>

      {error ? (
        <p
          role="alert"
          className="studio-shake flex items-start gap-2.5 border border-destructive bg-surface-2 px-4 py-3 text-body-sm text-destructive"
        >
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {error}
        </p>
      ) : null}
    </div>
  );
}
