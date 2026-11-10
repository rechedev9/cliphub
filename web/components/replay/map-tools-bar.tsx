'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ArrowUpRight, Brush, Eraser, FileText, Film, Hand, Timer, Trash2, Undo2 } from 'lucide-react';
import { STROKE_COLORS } from '@/lib/api/spatial';
import { STROKE_LABEL, STROKE_VAR, chooseDrawMode, drawMode } from '@/components/tactical/spatial/telestrator';
import type { DrawMode, TelestratorDraft } from '@/components/tactical/spatial/telestrator';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ReplayChip } from './replay-chip';

/** The chips name themselves where the row has room; in a narrow one the icon and its tooltip do. */
const WIDE_LABEL = 'hidden @4xl:inline';

const DRAW_MODES: readonly { mode: DrawMode; label: string; hint: string; icon: ReactNode }[] = [
  { mode: 'move', label: 'Mover', hint: 'Mover el mapa (H)', icon: <Hand aria-hidden /> },
  { mode: 'draw', label: 'Dibujar', hint: 'Dibujar (D)', icon: <Brush aria-hidden /> },
  { mode: 'arrow', label: 'Flecha', hint: 'Flecha (A)', icon: <ArrowUpRight aria-hidden /> },
  { mode: 'erase', label: 'Borrar trazo', hint: 'Borrar trazo: pulsa o arrastra sobre un trazo del dibujo para quitarlo (E)', icon: <Eraser aria-hidden /> },
];

function IconButton({ label, disabled = false, onClick, children }: { label: string; disabled?: boolean; onClick: () => void; children: ReactNode }): ReactNode {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={cn('grid size-7 place-items-center rounded-sm text-fg-2 outline-none hover:bg-surface-3 hover:text-fg-1 disabled:pointer-events-none disabled:text-fg-4 [&_svg]:size-3.5', FOCUS_RING)}
    >
      {children}
    </button>
  );
}

/** Mover, Dibujar and Flecha, the stroke colours and undo: the one drawing bar of the Replay. */
function DrawTools({ draft }: { draft: TelestratorDraft }): ReactNode {
  const mode = drawMode(draft);
  return (
    <div role="toolbar" aria-label="Dibujo sobre el mapa" className="flex items-center gap-1">
      {DRAW_MODES.map((entry) => (
        <ReplayChip key={entry.mode} pressed={mode === entry.mode} onClick={() => chooseDrawMode(draft, entry.mode)} title={entry.hint} aria-label={entry.label}>
          {entry.icon}
          <span className={WIDE_LABEL}>{entry.label}</span>
        </ReplayChip>
      ))}
      <ReplayChip
        pressed={draft.tool.timed}
        onClick={() => draft.setTool({ ...draft.tool, timed: !draft.tool.timed })}
        title="Temporal: cada trazo se desvanece solo a los 4 s y no entra en la nota (T)"
        aria-label="Trazo temporal"
      >
        <Timer aria-hidden />
        <span className={WIDE_LABEL}>Temporal</span>
      </ReplayChip>
      <div className="flex items-center gap-1 px-1" role="group" aria-label="Color del trazo">
        {STROKE_COLORS.map((color) => (
          <button
            key={color}
            type="button"
            aria-label={STROKE_LABEL[color]}
            title={STROKE_LABEL[color]}
            aria-pressed={draft.tool.color === color}
            onClick={() => draft.setTool({ ...draft.tool, color })}
            className={cn('size-5 rounded-full border-2 outline-none', FOCUS_RING, draft.tool.color === color ? 'border-fg-1' : 'border-transparent')}
            style={{ backgroundColor: STROKE_VAR[color] }}
          />
        ))}
      </div>
      <IconButton label="Deshacer el último trazo (Z)" disabled={draft.strokes.length === 0} onClick={draft.undo}>
        <Undo2 aria-hidden />
      </IconButton>
      <IconButton label="Borrar todo el dibujo" disabled={draft.strokes.length === 0} onClick={draft.clear}>
        <Trash2 aria-hidden />
      </IconButton>
    </div>
  );
}

/** Where the demo of the round on screen is described: its date, when the page knows it, and its record. */
export type DemoLink = { date: string | null; href: string };

/**
 * The row under the map: the drawing tools, the GIF of the tramo, the demo the
 * round comes from and the map's zoom buttons.
 */
export function MapToolsBar({
  draft,
  gif,
  demo,
  zoomSlot,
}: {
  /** The drawing of the note being made; null where notes cannot be saved. */
  draft: TelestratorDraft | null;
  /** The GIF row over this one, opened with this chip or G. */
  gif: { open: boolean; onToggle: () => void };
  demo: DemoLink | null;
  /** Where the map puts its zoom buttons. */
  zoomSlot: (element: HTMLDivElement | null) => void;
}): ReactNode {
  return (
    <div className="@container flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1.5 border-t border-border-subtle bg-surface-1 px-2 py-1.5">
      {draft === null ? null : <DrawTools draft={draft} />}
      <ReplayChip pressed={gif.open} aria-expanded={gif.open} onClick={gif.onToggle} title="Exportar el tramo como GIF o MP4 (G)" aria-label="Exportar el tramo">
        <Film aria-hidden />
        GIF / MP4
      </ReplayChip>
      <div className="ml-auto flex min-w-0 items-center gap-2">
        {demo === null ? null : (
          <Link
            href={demo.href}
            className={cn('flex min-w-0 items-center gap-1.5 rounded-sm px-1 text-meta text-fg-2 outline-none hover:text-fg-1', FOCUS_RING)}
          >
            <FileText aria-hidden className="size-3.5 shrink-0" />
            <span className="truncate">{demo.date === null ? 'Detalles de la demo' : `${demo.date} · Detalles de la demo`}</span>
          </Link>
        )}
        <div ref={zoomSlot} />
      </div>
    </div>
  );
}
