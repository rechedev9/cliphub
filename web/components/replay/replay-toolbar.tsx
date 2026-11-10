'use client';

import type { ComponentProps, ReactNode, RefObject } from 'react';
import { ArrowUpRight, Brush, Eraser, Film, Hand, Keyboard, Settings, Timer, Trash2, Undo2, X } from 'lucide-react';
import { STROKE_COLORS } from '@/lib/api/spatial';
import { STROKE_LABEL, STROKE_VAR, chooseDrawMode, drawMode } from '@/components/tactical/spatial/telestrator';
import type { DrawMode, TelestratorDraft } from '@/components/tactical/spatial/telestrator';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ReplayChip } from './replay-chip';

/** An icon button of the toolbar: square, the active one outlined in white as in EDGE. */
function ToolButton({ className, children, ...props }: ComponentProps<typeof ReplayChip>): ReactNode {
  return (
    <ReplayChip className={cn('size-7 justify-center px-0', props.pressed && 'border-fg-1', className)} {...props}>
      {children}
    </ReplayChip>
  );
}

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

/** What the pencil opens: the arrow, timed strokes, the colours, undo and clear. Shown while a drawing tool is chosen. */
function DrawOptions({ draft, mode }: { draft: TelestratorDraft; mode: DrawMode }): ReactNode {
  return (
    <div role="group" aria-label="Opciones del dibujo" className="flex items-center gap-1 border-l border-border-subtle pl-2">
      <ReplayChip pressed={mode === 'arrow'} onClick={() => chooseDrawMode(draft, mode === 'arrow' ? 'draw' : 'arrow')} title="Flecha (A)" aria-label="Flecha">
        <ArrowUpRight aria-hidden />
        Flecha
      </ReplayChip>
      <ReplayChip
        pressed={draft.tool.timed}
        onClick={() => draft.setTool({ ...draft.tool, timed: !draft.tool.timed })}
        title="Temporal: cada trazo se desvanece solo a los 4 s y no entra en la nota (T)"
        aria-label="Trazo temporal"
      >
        <Timer aria-hidden />
        Temporal
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

/**
 * The row of icon buttons over the round chips: Ajustes, Atajos, Mover,
 * Dibujar, Borrar and GIF / MP4, with the player the map follows. The pencil's
 * colours and timed strokes open beside it while a drawing tool is chosen.
 */
export function ReplayToolbar({
  draft,
  gif,
  preferences,
  onHelp,
  follow,
}: {
  /** The drawing of the note being made; null where notes cannot be saved. */
  draft: TelestratorDraft | null;
  /** The export row over the map, opened with this button or G. */
  gif: { open: boolean; onToggle: () => void };
  /** Preferencias del replay, a dialog over the replay. */
  preferences: { open: boolean; onOpen: () => void; triggerRef: RefObject<HTMLButtonElement | null> };
  onHelp: () => void;
  /** The player the map keeps centred. */
  follow: { name: string; onStop: () => void } | null;
}): ReactNode {
  const mode = draft === null ? 'move' : drawMode(draft);
  return (
    <div role="toolbar" aria-label="Herramientas del replay" className="flex min-w-0 flex-wrap items-center gap-1">
      <ToolButton
        ref={preferences.triggerRef}
        pressed={preferences.open}
        aria-haspopup="dialog"
        aria-expanded={preferences.open}
        onClick={preferences.onOpen}
        title="Ajustes: preferencias del replay"
        aria-label="Ajustes del replay"
      >
        <Settings aria-hidden />
      </ToolButton>
      <ToolButton pressed={false} onClick={onHelp} title="Atajos de teclado (?)" aria-label="Atajos de teclado">
        <Keyboard aria-hidden />
      </ToolButton>
      {draft === null ? null : (
        <>
          <ToolButton pressed={mode === 'move'} onClick={() => chooseDrawMode(draft, 'move')} title="Mover el mapa (H)" aria-label="Mover">
            <Hand aria-hidden />
          </ToolButton>
          <ToolButton pressed={mode === 'draw' || mode === 'arrow'} onClick={() => chooseDrawMode(draft, 'draw')} title="Dibujar (D)" aria-label="Dibujar">
            <Brush aria-hidden />
          </ToolButton>
          <ToolButton
            pressed={mode === 'erase'}
            onClick={() => chooseDrawMode(draft, 'erase')}
            title="Borrar trazo: pulsa o arrastra sobre un trazo del dibujo para quitarlo (E)"
            aria-label="Borrar trazo"
          >
            <Eraser aria-hidden />
          </ToolButton>
        </>
      )}
      <ToolButton pressed={gif.open} aria-expanded={gif.open} onClick={gif.onToggle} title="Exportar el tramo como GIF o MP4 (G)" aria-label="Exportar el tramo como GIF o MP4">
        <Film aria-hidden />
      </ToolButton>
      {draft === null || mode === 'move' ? null : <DrawOptions draft={draft} mode={mode} />}
      {follow === null ? null : (
        <button
          type="button"
          onClick={follow.onStop}
          className={cn('ml-1 flex h-7 items-center gap-1.5 rounded-sm border border-border-strong bg-surface-1 px-2 text-meta text-fg-1 outline-none hover:bg-surface-3', FOCUS_RING)}
        >
          Siguiendo a <span className="max-w-32 truncate font-semibold">{follow.name}</span>
          <X className="size-3.5 text-fg-3" aria-label="Dejar de seguir" />
        </button>
      )}
    </div>
  );
}
