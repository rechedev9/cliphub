'use client';

import { useCallback, useId, useState } from 'react';
import { AlertTriangle, CloudUpload, LockKeyhole, UserRoundX } from 'lucide-react';
import { MAX_DEMO_FILES } from '@/lib/upload/demo-names';
import { expandDemoUploads } from '@/lib/upload/expand-archives';
import { cn } from '@/lib/utils';

export type DemoDropzoneProps = {
  /** Called with the chosen .dem file(s). The parent owns parsing + navigation. */
  onFiles: (files: File[]) => void;
  compact?: boolean;
  disabled?: boolean;
};

/** Drop zone for .dem / archives; the label opens the native file dialog. */
export function DemoDropzone({ onFiles, compact = false, disabled = false }: DemoDropzoneProps) {
  const inputId = useId();
  const [dragging, setDragging] = useState(false);
  const [extracting, setExtracting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const busy = extracting || disabled;

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
        .finally(() => setExtracting(false));
    },
    [busy, onFiles],
  );

  return (
    <div className="flex flex-col gap-3">
      <label
        htmlFor={inputId}
        data-dragging={dragging ? 'true' : undefined}
        data-layout={compact ? 'compact' : 'full'}
        onDragOver={(e) => {
          e.preventDefault();
          if (!busy) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          if (!busy) accept(e.dataTransfer.files);
        }}
        aria-busy={busy || undefined}
        aria-disabled={disabled || undefined}
        className={cn(
          'studio-panel studio-panel-raised group relative isolate flex cursor-pointer flex-col items-center justify-center overflow-hidden text-center',
          compact ? 'min-h-24 px-4 py-5' : 'min-h-[24rem] px-6 pt-10 pb-32',
          busy && 'pointer-events-none cursor-wait',
          '[perspective:var(--perspective)] [perspective-origin:50%_40%]',
          'transition-[border-color,box-shadow,transform] duration-(--dur-base) ease-standard',
          !compact && '@[40rem]/upload:min-h-[22rem] @[40rem]/upload:px-10 @[40rem]/upload:pt-12 @[40rem]/upload:pb-20',
          'has-[:focus-visible]:border-primary has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
          dragging
            ? 'border-primary shadow-[var(--elev-4),var(--glow-primary-lg)]'
            : 'hover:border-primary/60 hover:shadow-[var(--elev-2),var(--glow-primary-sm)]',
        )}
      >
        {/* The dashed sheet the icon hovers over. */}
        <span
          aria-hidden
          className={cn(
            'pointer-events-none absolute inset-2 border border-dashed transition-colors duration-(--dur-base) ease-standard',
            dragging ? 'border-primary/90 bg-primary/6' : 'border-primary/30 group-hover:border-primary/60',
          )}
        />
        <span
          aria-hidden
          className="pointer-events-none absolute inset-x-[10%] top-0 h-52 opacity-70 transition-opacity duration-(--dur-base) ease-standard group-hover:opacity-100"
          style={{
            background:
              'radial-gradient(ellipse at top, color-mix(in oklch, var(--primary) 8%, transparent), transparent 72%)',
          }}
        />

        {compact ? null : (
          <span className="relative z-10 mb-5 inline-flex items-center gap-3 font-mono text-meta uppercase tracking-wider text-primary">
            <span aria-hidden className="h-px w-6 bg-primary/65" />
            Demo de CS2
            <span aria-hidden className="h-px w-6 bg-primary/65" />
          </span>
        )}

        {/* The lifting plane: the icon rises on translateZ and its contact
            shadow stays behind on the sheet, so the gap between them is what
            reads as height. */}
        <span className="relative z-10 grid place-items-center [transform-style:preserve-3d]">
          <span
            aria-hidden
            className={cn(
              'col-start-1 row-start-1 rounded-full bg-[radial-gradient(circle,oklch(0.02_0.02_264/0.8),transparent_70%)] blur-[5px]',
              compact ? 'size-10' : 'size-16 @[40rem]/upload:size-[4.5rem]',
              'transition-[opacity,transform] duration-(--dur-base) ease-standard',
              dragging
                ? 'opacity-90 [transform:translateY(1.15rem)_scale(0.85)]'
                : 'opacity-0 [transform:translateY(0.4rem)_scale(0.7)]',
            )}
          />
          <span
            className={cn(
              'col-start-1 row-start-1 flex items-center justify-center rounded-full border border-primary/55 bg-surface-0 text-primary',
              compact ? 'size-10' : 'size-16 @[40rem]/upload:size-[4.5rem]',
              'shadow-[0_0_32px_color-mix(in_oklch,var(--primary)_22%,transparent),inset_0_0_18px_color-mix(in_oklch,var(--primary)_12%,transparent)]',
              'transition-transform duration-(--dur-base) ease-standard',
              dragging
                ? '[transform:translateZ(calc(var(--shell-depth)*52px))_scale(1.06)]'
                : 'group-hover:[transform:translateZ(calc(var(--shell-depth)*18px))]',
            )}
          >
            <CloudUpload className={compact ? 'size-5' : 'size-7 @[40rem]/upload:size-8'} strokeWidth={1.7} />
          </span>
        </span>

        <span
          className={cn(
            'relative z-10 font-display font-bold text-fg-1',
            compact ? 'mt-3 text-body' : 'mt-5 text-section @[40rem]/upload:text-display-sm',
          )}
        >
          SUELTA UN .DEM AQUÍ
        </span>
        {compact ? null : (
          <span className="relative z-10 mt-2 max-w-lg text-body text-fg-2">
            {extracting
              ? 'Extrayendo archivo…'
              : 'Arrastra una demo .dem, .dem.zst o un .rar/.zip de la serie'}
          </span>
        )}
        <span
          className={cn(
            'relative z-10 inline-flex min-h-11 items-center justify-center border border-primary/65 bg-primary/8 font-display text-body-sm font-semibold uppercase tracking-wide text-primary transition-colors duration-(--dur-fast) ease-standard group-hover:border-primary group-hover:bg-primary/14',
            compact ? 'mt-3 px-5' : 'mt-5 px-8',
          )}
        >
          explora tus archivos
        </span>

        {compact ? null : (
          <span className="absolute inset-x-2 bottom-2 z-10 grid min-h-24 grid-cols-1 items-center gap-2 border-t border-border bg-surface-0/40 px-5 py-3 font-mono text-meta uppercase tracking-wider text-fg-3 @[40rem]/upload:min-h-14 @[40rem]/upload:grid-cols-3 @[40rem]/upload:gap-0 @[40rem]/upload:px-8">
            <span className="inline-flex items-center justify-center gap-2 @[40rem]/upload:border-r @[40rem]/upload:border-border">
              <UserRoundX aria-hidden className="size-4" />
              Sin login
            </span>
            <span className="inline-flex items-center justify-center gap-2 @[40rem]/upload:border-r @[40rem]/upload:border-border">
              <span className="tabular-nums text-primary">{MAX_DEMO_FILES}</span>
              demos máximo
            </span>
            <span className="inline-flex items-center justify-center gap-2">
              <LockKeyhole aria-hidden className="size-4" />
              El .dem no sale de tu PC
            </span>
          </span>
        )}

        <input
          id={inputId}
          type="file"
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
          className="flex items-start gap-2.5 border border-destructive/45 bg-destructive/8 px-4 py-3 text-body-sm text-destructive"
        >
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {error}
        </p>
      ) : null}
    </div>
  );
}
