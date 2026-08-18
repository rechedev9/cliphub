'use client';

import { type ReactElement } from 'react';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  deleteItem,
  duplicateItem,
  removeTransition,
  setCanvas,
  setItemProps,
  setMusic,
  setTransitionAfter,
} from '@/lib/editor/document';
import {
  EDITOR_CANVAS,
  EDITOR_FILTERS,
  EDITOR_TRANSITIONS,
  resolvedTransform,
  type EditorDocument,
  type EditorFilter,
  type EditorItem,
  type EditorTransform,
  type EditorTransition,
} from '@/lib/editor/evaluate';
import { EDITOR_LIMITS } from '@/lib/editor/validate';

const NONE_VALUE = '__none__';
const DEFAULT_CROSSFADE = 0.25;
const CANVAS_PORTRAIT = 'portrait';
const CANVAS_LANDSCAPE = 'landscape';

const TRANSFORM_PRESETS = [
  { id: 'full', label: 'Pantalla completa', transform: { x: 0, y: 0, width: 1, height: 1 } },
  { id: 'pip', label: 'PiP esquina', transform: { x: 0.62, y: 0.72, width: 0.35, height: 0.25 } },
  { id: 'top', label: 'Mitad superior', transform: { x: 0, y: 0, width: 1, height: 0.5 } },
  { id: 'bottom', label: 'Mitad inferior', transform: { x: 0, y: 0.5, width: 1, height: 0.5 } },
] as const;

export type EditorInspectorProps = {
  doc: EditorDocument;
  selectedId: string | null;
  locked: boolean;
  songs: ReadonlyArray<{ id: string; title: string }>;
  onChange: (next: EditorDocument) => void;
};

export function EditorInspector({
  doc,
  selectedId,
  locked,
  songs,
  onChange,
}: EditorInspectorProps): ReactElement {
  const selected = selectedId === null ? null : findItem(doc, selectedId);
  const canvas = canvasOrientation(doc);
  const musicKey = doc.music?.key ?? '';
  const musicVolume = doc.music?.volume ?? 1;

  return (
    <aside className="studio-panel flex min-h-0 min-w-0 flex-col overflow-auto p-4">
      <div className="flex flex-col gap-4">
        <section className="flex flex-col gap-4">
          <h2 className="font-mono text-meta uppercase tracking-wider text-fg-3">Proyecto</h2>
          <Field label="Lienzo">
            {(control) => (
              <ToggleGroup
                {...control}
                type="single"
                variant="filter"
                size="default"
                value={canvas}
                disabled={locked}
                onValueChange={(value) => {
                  if (!isCanvasOrientation(value)) return;
                  onChange(setCanvas(doc, value));
                }}
              >
                <ToggleGroupItem value={CANVAS_PORTRAIT}>Vertical</ToggleGroupItem>
                <ToggleGroupItem value={CANVAS_LANDSCAPE}>Horizontal</ToggleGroupItem>
              </ToggleGroup>
            )}
          </Field>
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-2">
              <span aria-hidden className="size-1.5 shrink-0 bg-stream" />
              <h3 className="text-label uppercase tracking-wide text-stream-text">Música</h3>
              <StatusTag tone="stream" size="sm">
                {musicKey === '' ? 'Sin pista' : `${Math.round(musicVolume * 100)}%`}
              </StatusTag>
            </div>
            <Field label="Pista">
              {(control) => (
                <Select
                  value={musicKey === '' ? NONE_VALUE : musicKey}
                  disabled={locked}
                  onValueChange={(value) => {
                    const key = value === NONE_VALUE ? undefined : value;
                    onChange(setMusic(doc, key, musicVolume));
                  }}
                >
                  <SelectTrigger {...control}>
                    <SelectValue placeholder="Ninguna" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE_VALUE}>Ninguna</SelectItem>
                    {songs.map((song) => (
                      <SelectItem key={song.id} value={song.id}>
                        {song.title}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </Field>
            <NumberField
              label="Volumen de música"
              value={musicVolume}
              min={0}
              max={EDITOR_LIMITS.musicVolumeMax}
              step={0.05}
              disabled={locked}
              onCommit={(volume) => onChange(setMusic(doc, musicKey === '' ? undefined : musicKey, volume))}
            />
          </div>
        </section>

        {selected === null ? (
          <p className="text-body-sm text-fg-2">Selecciona un clip del timeline.</p>
        ) : (
          <ItemInspector doc={doc} item={selected} locked={locked} onChange={onChange} />
        )}
      </div>
    </aside>
  );
}

function ItemInspector({
  doc,
  item,
  locked,
  onChange,
}: {
  doc: EditorDocument;
  item: EditorItem;
  locked: boolean;
  onChange: (next: EditorDocument) => void;
}): ReactElement {
  const transform = resolvedTransform(item);
  const filterValue = item.filter === EDITOR_FILTERS.grade ? EDITOR_FILTERS.grade : NONE_VALUE;
  const transition = transitionAfter(doc, item.id);

  function patch(next: Partial<EditorItem>): void {
    onChange(setItemProps(doc, item.id, next));
  }

  function patchTransform(next: Partial<EditorTransform>): void {
    patch({ transform: { ...transform, ...next } });
  }

  return (
    <section className="flex flex-col gap-4 border-t border-border pt-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="font-mono text-meta uppercase tracking-wider text-fg-3">Clip</h2>
        <span className="truncate font-mono text-meta text-fg-2">{item.id}</span>
      </div>
      <NumberField
        label="Inicio"
        value={item.timeline_start}
        min={0}
        step={0.05}
        disabled={locked}
        onCommit={(timeline_start) => patch({ timeline_start })}
      />
      <NumberField
        label="Entrada"
        value={item.source_in}
        min={0}
        step={0.05}
        disabled={locked}
        onCommit={(source_in) => patch({ source_in })}
      />
      <NumberField
        label="Salida"
        value={item.source_out}
        min={0}
        step={0.05}
        disabled={locked}
        onCommit={(source_out) => patch({ source_out })}
      />
      <NumberField
        label="Velocidad"
        value={item.speed ?? 1}
        min={EDITOR_LIMITS.minSpeed}
        max={EDITOR_LIMITS.maxSpeed}
        step={0.05}
        disabled={locked}
        onCommit={(speed) => patch({ speed })}
      />
      <NumberField
        label="Volumen"
        value={item.volume ?? 1}
        min={0}
        max={EDITOR_LIMITS.maxVolume}
        step={0.05}
        disabled={locked}
        onCommit={(volume) => patch({ volume })}
      />
      <NumberField
        label="Fundido de entrada"
        value={item.fade_in ?? 0}
        min={0}
        max={EDITOR_LIMITS.maxFadeSeconds}
        step={0.05}
        disabled={locked}
        onCommit={(fade_in) => patch({ fade_in })}
      />
      <NumberField
        label="Fundido de salida"
        value={item.fade_out ?? 0}
        min={0}
        max={EDITOR_LIMITS.maxFadeSeconds}
        step={0.05}
        disabled={locked}
        onCommit={(fade_out) => patch({ fade_out })}
      />

      <div className="flex flex-col gap-4">
        <h3 className="font-mono text-meta uppercase tracking-wider text-fg-3">Transformación</h3>
        <div className="grid grid-cols-2 gap-4">
          <NumberField label="X" value={transform.x} min={0} max={1} step={0.01} disabled={locked} onCommit={(x) => patchTransform({ x })} />
          <NumberField label="Y" value={transform.y} min={0} max={1} step={0.01} disabled={locked} onCommit={(y) => patchTransform({ y })} />
          <NumberField
            label="Ancho"
            value={transform.width}
            min={0.01}
            max={1}
            step={0.01}
            disabled={locked}
            onCommit={(width) => patchTransform({ width })}
          />
          <NumberField
            label="Alto"
            value={transform.height}
            min={0.01}
            max={1}
            step={0.01}
            disabled={locked}
            onCommit={(height) => patchTransform({ height })}
          />
        </div>
        <NumberField
          label="Opacidad"
          value={transform.opacity ?? 1}
          min={0}
          max={1}
          step={0.05}
          disabled={locked}
          onCommit={(opacity) => patchTransform({ opacity })}
        />
        <div className="grid grid-cols-2 gap-2">
          {TRANSFORM_PRESETS.map((preset) => (
            <Button
              key={preset.id}
              type="button"
              variant="outline"
              size="sm"
              disabled={locked}
              onClick={() => patchTransform(preset.transform)}
            >
              {preset.label}
            </Button>
          ))}
        </div>
      </div>

      <Field label="Filtro">
        {(control) => (
          <Select
            value={filterValue}
            disabled={locked}
            onValueChange={(value) => {
              const filter: EditorFilter = value === EDITOR_FILTERS.grade ? EDITOR_FILTERS.grade : EDITOR_FILTERS.none;
              patch({ filter });
            }}
          >
            <SelectTrigger {...control}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE_VALUE}>Ninguno</SelectItem>
              <SelectItem value={EDITOR_FILTERS.grade}>Grade</SelectItem>
            </SelectContent>
          </Select>
        )}
      </Field>

      <Field label="Transición">
        {(control) => (
          <Select
            value={transition.kind}
            disabled={locked}
            onValueChange={(value) => {
              if (value === EDITOR_TRANSITIONS.crossfade) {
                onChange(setTransitionAfter(doc, item.id, EDITOR_TRANSITIONS.crossfade, transition.duration));
                return;
              }
              if (transition.id === undefined) return;
              onChange(removeTransition(doc, transition.id));
            }}
          >
            <SelectTrigger {...control}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={EDITOR_TRANSITIONS.cut}>Corte</SelectItem>
              <SelectItem value={EDITOR_TRANSITIONS.crossfade}>Fundido cruzado</SelectItem>
            </SelectContent>
          </Select>
        )}
      </Field>
      {transition.kind === EDITOR_TRANSITIONS.crossfade ? (
        <NumberField
          label="Duración"
          value={transition.duration}
          min={0.05}
          max={EDITOR_LIMITS.maxFadeSeconds}
          step={0.05}
          disabled={locked}
          onCommit={(duration) => onChange(setTransitionAfter(doc, item.id, EDITOR_TRANSITIONS.crossfade, duration))}
        />
      ) : null}

      <div className="flex flex-col gap-2">
        <Button type="button" variant="outline" disabled={locked} onClick={() => onChange(duplicateItem(doc, item.id))}>
          Duplicar
        </Button>
        <Button type="button" variant="destructive" disabled={locked} onClick={() => onChange(deleteItem(doc, item.id))}>
          Eliminar
        </Button>
      </div>
    </section>
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

function findItem(doc: EditorDocument, id: string): EditorItem | null {
  for (const track of doc.tracks) {
    for (const item of track.items) {
      if (item.id === id) return item;
    }
  }
  return null;
}

function canvasOrientation(doc: EditorDocument): 'portrait' | 'landscape' {
  if (doc.canvas.width === EDITOR_CANVAS.landscape.width && doc.canvas.height === EDITOR_CANVAS.landscape.height) {
    return CANVAS_LANDSCAPE;
  }
  return CANVAS_PORTRAIT;
}

function isCanvasOrientation(value: string): value is 'portrait' | 'landscape' {
  return value === CANVAS_PORTRAIT || value === CANVAS_LANDSCAPE;
}

type TransitionView = {
  id?: string;
  kind: EditorTransition['kind'];
  duration: number;
};

function transitionAfter(doc: EditorDocument, itemId: string): TransitionView {
  const found = (doc.transitions ?? []).find((entry) => entry.after_item === itemId);
  if (found === undefined || found.kind !== EDITOR_TRANSITIONS.crossfade) {
    return { id: found?.id, kind: EDITOR_TRANSITIONS.cut, duration: DEFAULT_CROSSFADE };
  }
  const duration =
    found.duration !== undefined && Number.isFinite(found.duration) && found.duration > 0
      ? found.duration
      : DEFAULT_CROSSFADE;
  return { id: found.id, kind: EDITOR_TRANSITIONS.crossfade, duration };
}

function clamp(value: number, min: number | undefined, max: number | undefined): number {
  let next = value;
  if (min !== undefined && next < min) next = min;
  if (max !== undefined && next > max) next = max;
  return next;
}
