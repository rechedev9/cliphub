'use client';

import { Plus, Trash2 } from 'lucide-react';
import { type ReactElement } from 'react';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { addOverlay, deleteOverlay, updateOverlay } from '@/lib/editor/document';
import { type EditorDocument, type EditorTextOverlay } from '@/lib/editor/evaluate';
import { EDITOR_LIMITS } from '@/lib/editor/validate';

const DEFAULT_FONT_SIZE = 64;
const DEFAULT_POSITION_Y = 0.1;
const DEFAULT_OVERLAY_TEXT = 'TEXTO';

export function EditorOverlaysPanel({
  doc,
  locked,
  onChange,
}: {
  doc: EditorDocument;
  locked: boolean;
  onChange: (d: EditorDocument) => void;
}): ReactElement {
  const overlays = doc.overlays ?? [];
  const atCap = overlays.length >= EDITOR_LIMITS.maxOverlays;

  return (
    <aside className="studio-panel flex min-h-0 min-w-0 flex-col gap-4 overflow-auto p-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="font-mono text-meta tracking-wider text-fg-3 uppercase">
          Textos · {overlays.length}/{EDITOR_LIMITS.maxOverlays}
        </h2>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={locked || atCap}
          onClick={() =>
            onChange(
              addOverlay(doc, {
                text: DEFAULT_OVERLAY_TEXT,
                position_y: DEFAULT_POSITION_Y,
                start_seconds: 0,
                font_size: DEFAULT_FONT_SIZE,
              }),
            )
          }
        >
          <Plus className="size-4" aria-hidden />
          Añadir texto
        </Button>
      </div>
      {overlays.length === 0 ? <p className="text-body-sm text-fg-3">No hay textos en el timeline.</p> : null}
      {overlays.map((overlay) => (
        <OverlayEditor
          key={overlay.id}
          overlay={overlay}
          locked={locked}
          onPatch={(patch) => onChange(updateOverlay(doc, overlay.id, patch))}
          onDelete={() => onChange(deleteOverlay(doc, overlay.id))}
        />
      ))}
    </aside>
  );
}

function OverlayEditor({
  overlay,
  locked,
  onPatch,
  onDelete,
}: {
  overlay: EditorTextOverlay;
  locked: boolean;
  onPatch: (patch: Partial<Omit<EditorTextOverlay, 'id'>>) => void;
  onDelete: () => void;
}): ReactElement {
  return (
    <div className="flex flex-col gap-3 border border-border-subtle bg-surface-1 p-3">
      <div className="flex items-end gap-2">
        <Field label="Texto" className="min-w-0 flex-1">
          {(control) => (
            <Input
              {...control}
              value={overlay.text}
              maxLength={EDITOR_LIMITS.maxTextRunes}
              disabled={locked}
              onChange={(event) => onPatch({ text: event.target.value.slice(0, EDITOR_LIMITS.maxTextRunes) })}
            />
          )}
        </Field>
        <Button type="button" variant="ghost" size="icon-sm" disabled={locked} onClick={onDelete} aria-label="Eliminar texto">
          <Trash2 className="size-4" aria-hidden />
        </Button>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <NumberField
          label="Inicio (s)"
          value={overlay.start_seconds}
          min={0}
          step={0.05}
          disabled={locked}
          onCommit={(start_seconds) => onPatch({ start_seconds })}
        />
        <Field label="Fin (s)">
          {(control) => (
            <Input
              {...control}
              type="number"
              inputMode="decimal"
              className="font-mono"
              min={0}
              step={0.05}
              disabled={locked}
              value={overlay.end_seconds ?? ''}
              onChange={(event) => {
                const raw = event.target.value;
                if (raw === '') {
                  onPatch({ end_seconds: undefined });
                  return;
                }
                const next = Number(raw);
                if (Number.isFinite(next)) onPatch({ end_seconds: next });
              }}
            />
          )}
        </Field>
        <NumberField
          label="Posición Y"
          value={overlay.position_y}
          min={EDITOR_LIMITS.minVerticalCenterY}
          max={EDITOR_LIMITS.maxVerticalCenterY}
          step={0.005}
          disabled={locked}
          onCommit={(position_y) => onPatch({ position_y })}
        />
        <NumberField
          label="Tamaño"
          value={overlay.font_size ?? DEFAULT_FONT_SIZE}
          min={EDITOR_LIMITS.minFontSize}
          max={EDITOR_LIMITS.maxFontSize}
          step={1}
          disabled={locked}
          onCommit={(font_size) => onPatch({ font_size })}
        />
      </div>
    </div>
  );
}

function NumberField({
  label,
  value,
  min,
  max,
  step,
  disabled,
  onCommit,
}: {
  label: string;
  value: number;
  min?: number;
  max?: number;
  step?: number;
  disabled: boolean;
  onCommit: (value: number) => void;
}): ReactElement {
  return (
    <Field label={label}>
      {(control) => (
        <Input
          {...control}
          type="number"
          inputMode="decimal"
          className="font-mono"
          value={Number.isFinite(value) ? value : 0}
          min={min}
          max={max}
          step={step ?? 0.05}
          disabled={disabled}
          onChange={(event) => {
            const next = Number(event.target.value);
            if (!Number.isFinite(next)) return;
            onCommit(clamp(next, min, max));
          }}
        />
      )}
    </Field>
  );
}

function clamp(value: number, min: number | undefined, max: number | undefined): number {
  let next = value;
  if (min !== undefined && next < min) next = min;
  if (max !== undefined && next > max) next = max;
  return next;
}
