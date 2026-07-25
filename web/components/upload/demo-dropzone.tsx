'use client';

import { useCallback, useId, useState } from 'react';
import { AlertTriangle, CloudUpload, LockKeyhole, UserRoundX } from 'lucide-react';
import { cn } from '@/lib/utils';

export type DemoDropzoneProps = {
  /** Called with the chosen .dem file(s). The parent owns parsing + navigation. */
  onFiles: (files: File[]) => void;
};

const DEM_EXT = '.dem';

/** Most demos we ever record for one series is a bo5 (5 maps); 10 leaves slack. */
const MAX_FILES = 10;

/**
 * A drop zone + file picker for CS2 .dem files, styled as a restrained
 * workstation target with a dashed cyan inset and a dedicated trust rail. It
 * accepts a single demo or several at once — a whole bo3/bo5 series — up to
 * {@link MAX_FILES}. The clickable area is a <label> bound to the file input, so
 * the OS file dialog opens natively on click (no JS .click() that can be flaky
 * with hidden inputs). Drag-and-drop and keyboard both work; every file's
 * extension is validated before handing the list up.
 *
 * Depth: the label is a perspective box and the badge stack is a 3D plane, so
 * dragging a demo over it physically lifts the glowing icon off the dashed sheet
 * and drops a contact shadow onto it. Both the lift and the shadow are scaled by
 * `--shell-depth`, so the efficiency profile, an inactive window, reduced motion
 * and forced colours all flatten them to nothing without a second rule here.
 */
export function DemoDropzone({ onFiles }: DemoDropzoneProps) {
  const inputId = useId();
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const accept = useCallback(
    (fileList: FileList | null | undefined) => {
      const files = fileList ? Array.from(fileList) : [];
      if (files.length === 0) return;
      if (files.length > MAX_FILES) {
        setError(`Máximo ${MAX_FILES} demos por serie. Has soltado ${files.length}.`);
        return;
      }
      const bad = files.find((f) => !f.name.toLowerCase().endsWith(DEM_EXT));
      if (bad) {
        setError(`"${bad.name}" no es una demo .dem.`);
        return;
      }
      setError(null);
      onFiles(files);
    },
    [onFiles],
  );

  return (
    <div className="flex flex-col gap-3">
      <label
        htmlFor={inputId}
        data-dragging={dragging ? 'true' : undefined}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          accept(e.dataTransfer.files);
        }}
        className={cn(
          'studio-panel studio-panel-raised group relative isolate flex min-h-[24rem] cursor-pointer flex-col items-center justify-center overflow-hidden px-6 pt-10 pb-32 text-center',
          '[perspective:var(--perspective)] [perspective-origin:50%_40%]',
          'transition-[border-color,box-shadow,transform] duration-(--dur-base) ease-standard',
          '@[40rem]/upload:min-h-[22rem] @[40rem]/upload:px-10 @[40rem]/upload:pt-12 @[40rem]/upload:pb-20',
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

        <span className="relative z-10 mb-5 inline-flex items-center gap-3 font-mono text-meta uppercase tracking-wider text-primary">
          <span aria-hidden className="h-px w-6 bg-primary/65" />
          Demo de CS2
          <span aria-hidden className="h-px w-6 bg-primary/65" />
        </span>

        {/* The lifting plane: the icon rises on translateZ and its contact
            shadow stays behind on the sheet, so the gap between them is what
            reads as height. */}
        <span className="relative z-10 grid place-items-center [transform-style:preserve-3d]">
          <span
            aria-hidden
            className={cn(
              'col-start-1 row-start-1 size-16 rounded-full bg-[radial-gradient(circle,oklch(0.02_0.02_264/0.8),transparent_70%)] blur-[5px]',
              'transition-[opacity,transform] duration-(--dur-base) ease-standard @[40rem]/upload:size-[4.5rem]',
              dragging
                ? 'opacity-90 [transform:translateY(1.15rem)_scale(0.85)]'
                : 'opacity-0 [transform:translateY(0.4rem)_scale(0.7)]',
            )}
          />
          <span
            className={cn(
              'col-start-1 row-start-1 flex size-16 items-center justify-center rounded-full border border-primary/55 bg-surface-0 text-primary @[40rem]/upload:size-[4.5rem]',
              'shadow-[0_0_32px_color-mix(in_oklch,var(--primary)_22%,transparent),inset_0_0_18px_color-mix(in_oklch,var(--primary)_12%,transparent)]',
              'transition-transform duration-(--dur-base) ease-standard',
              dragging
                ? '[transform:translateZ(calc(var(--shell-depth)*52px))_scale(1.06)]'
                : 'group-hover:[transform:translateZ(calc(var(--shell-depth)*18px))]',
            )}
          >
            <CloudUpload className="size-7 @[40rem]/upload:size-8" strokeWidth={1.7} />
          </span>
        </span>

        <span className="relative z-10 mt-5 font-display text-section font-bold text-fg-1 @[40rem]/upload:text-display-sm">
          SUELTA UN .DEM AQUÍ
        </span>
        <span className="relative z-10 mt-2 max-w-lg text-body text-fg-2">
          Arrastra una demo — o varias, una serie bo3/bo5 completa
        </span>
        <span className="relative z-10 mt-5 inline-flex min-h-11 items-center justify-center border border-primary/65 bg-primary/8 px-8 font-display text-body-sm font-semibold uppercase tracking-wide text-primary transition-colors duration-(--dur-fast) ease-standard group-hover:border-primary group-hover:bg-primary/14">
          explora tus archivos
        </span>

        <span className="absolute inset-x-2 bottom-2 z-10 grid min-h-24 grid-cols-1 items-center gap-2 border-t border-border bg-surface-0/40 px-5 py-3 font-mono text-meta uppercase tracking-wider text-fg-3 @[40rem]/upload:min-h-14 @[40rem]/upload:grid-cols-3 @[40rem]/upload:gap-0 @[40rem]/upload:px-8">
          <span className="inline-flex items-center justify-center gap-2 @[40rem]/upload:border-r @[40rem]/upload:border-border">
            <UserRoundX aria-hidden className="size-4" />
            Sin login
          </span>
          <span className="inline-flex items-center justify-center gap-2 @[40rem]/upload:border-r @[40rem]/upload:border-border">
            <span className="tabular-nums text-primary">{MAX_FILES}</span>
            demos máximo
          </span>
          <span className="inline-flex items-center justify-center gap-2">
            <LockKeyhole aria-hidden className="size-4" />
            El .dem no sale de tu PC
          </span>
        </span>

        <input
          id={inputId}
          type="file"
          multiple
          // No `accept` filter: on Windows the ".dem" filter hid every file in
          // folders without a .dem, so the dialog looked empty/broken. Show all
          // files; the extension check below rejects non-.dem with a message.
          className="sr-only"
          // Reset so picking the same file again still fires onChange.
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
