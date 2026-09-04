'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { ChevronDown, Plus, Trash2 } from 'lucide-react';
import type { StreamClipEdit, StreamClipRange, StreamTextOverlay } from '@/lib/api/streams';
import {
  CLIP_SPEEDS,
  DEFAULT_OVERLAY_FONT_SIZE,
  MAX_OVERLAY_FONT_SIZE,
  MAX_TEXT_OVERLAYS,
  MIN_OVERLAY_FONT_SIZE,
  streamRangeIssue,
  streamRangeOverlapIssue,
  withoutEmptyTextOverlays,
} from '@/lib/clip-edit';
import { STREAMER_BANNER_MAX_POSITION, STREAMER_BANNER_MIN_POSITION } from '@/lib/stream-preview';
import { clipOutputDuration, formatStreamClock, pruneClipEdit } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';
import { cn } from '@/lib/utils';

/** Both fades the chip toggles at once; the disclosure still edits each one. */
const CHIP_FADE_SECONDS = 0.5;

const CHIP_CLASS = 'font-mono uppercase tracking-wider';

/**
 * One card per cut. The card head selects the cut on the monitor; the chips
 * cover the everyday edits; the disclosure keeps every numeric field the
 * plan stores. Values written are exactly the ones the previous form wrote.
 */
export function StreamClipEditor({
  clips,
  sourceDuration,
  selectedClipId,
  onChange,
  onSelect,
  onRemove,
  disabled,
}: {
  clips: StreamClipRange[];
  sourceDuration: number;
  selectedClipId: string | null;
  onChange: (clips: StreamClipRange[]) => void;
  onSelect: (clip: StreamClipRange) => void;
  /** Owned by the editor so removing a cut also clears the monitor selection. */
  onRemove: (clip: StreamClipRange) => void;
  disabled: boolean;
}): ReactNode {
  const updateClip = (id: string, patch: Partial<StreamClipRange>) => {
    const next = clips.map((c) => (c.id === id ? { ...c, ...patch } : c));
    // A moved start must keep cuts in source order, same comparator as insertClipSorted.
    onChange(patch.start_seconds !== undefined ? [...next].sort((a, b) => a.start_seconds - b.start_seconds) : next);
  };
  const updateEdit = (id: string, patch: Partial<StreamClipEdit>) =>
    onChange(clips.map((c) => (c.id === id ? { ...c, edit: pruneClipEdit({ ...c.edit, ...patch }) } : c)));

  if (clips.length === 0) {
    return (
      <p className="border border-dashed border-border-subtle p-3.5 text-center text-body-sm text-fg-2">
        {sourceDuration > 0
          ? 'Haz clic en la timeline para añadir el primer corte. Cada corte sale como un Short independiente.'
          : 'Sin duración de la fuente: espera a que cargue el vídeo antes de añadir cortes.'}
      </p>
    );
  }

  return (
    <ul className="flex flex-col gap-2.5">
      {clips.map((clip, index) => (
        <ClipCard
          key={clip.id}
          clip={clip}
          index={index}
          sourceDuration={sourceDuration}
          overlapIssue={streamRangeOverlapIssue(clips, index)}
          selected={clip.id === selectedClipId}
          disabled={disabled}
          onSelect={() => onSelect(clip)}
          onRemove={() => onRemove(clip)}
          onClipChange={(patch) => updateClip(clip.id, patch)}
          onEditChange={(patch) => updateEdit(clip.id, patch)}
        />
      ))}
    </ul>
  );
}

function ClipCard({
  clip,
  index,
  sourceDuration,
  overlapIssue,
  selected,
  disabled,
  onSelect,
  onRemove,
  onClipChange,
  onEditChange,
}: {
  clip: StreamClipRange;
  index: number;
  sourceDuration: number;
  overlapIssue: string | null;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
  onRemove: () => void;
  onClipChange: (patch: Partial<StreamClipRange>) => void;
  onEditChange: (patch: Partial<StreamClipEdit>) => void;
}): ReactNode {
  const rangeIssue = streamRangeIssue(clip, sourceDuration, index) ?? overlapIssue;
  const speed = clip.edit?.speed ?? 1;
  const sourceVolume = clip.edit?.source_volume ?? 1;
  const fadeIn = clip.edit?.fade_in_seconds ?? 0;
  const fadeOut = clip.edit?.fade_out_seconds ?? 0;
  const fadesOn = fadeIn > 0 || fadeOut > 0;
  const overlays = clip.edit?.text_overlays ?? [];
  const number = String(index + 1).padStart(2, '0');
  const [rangeOpen, setRangeOpen] = useState(false);
  // Starts open when texts exist so clearing the last one to retype it does
  // not unmount the editor (and its blank row) mid-keystroke.
  const [textOpen, setTextOpen] = useState(overlays.length > 0);
  const showText = textOpen || overlays.length > 0;
  /** Volume the mute chip restores; written only when it mutes. */
  const previousVolumeRef = useRef(1);
  const toggleMute = (): void => {
    if (sourceVolume === 0) {
      onEditChange({ source_volume: previousVolumeRef.current });
      return;
    }
    previousVolumeRef.current = sourceVolume;
    onEditChange({ source_volume: 0 });
  };
  let cardClass = 'border-border bg-surface-2';
  if (rangeIssue !== null) cardClass = 'border-destructive/50';
  else if (selected) cardClass = 'border-stream/45 bg-surface-3';

  return (
    <li
      className={cn(
        'studio-enter flex flex-col gap-2.5 border p-3.5 transition-colors duration-(--dur-fast) ease-standard',
        cardClass,
      )}
    >
      <div className="flex items-center gap-2.5">
        <button
          type="button"
          onClick={onSelect}
          aria-pressed={selected}
          className="flex min-h-10 min-w-0 flex-1 items-center gap-2.5 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          {/* The index is an ordinal every card carries; only one card is selected. */}
          <span
            aria-hidden
            className={cn(
              'grid size-7 shrink-0 place-items-center border font-mono text-label tabular-nums',
              selected ? 'border-stream/45 bg-stream/10 text-stream-text' : 'border-border-strong text-fg-2',
            )}
          >
            {number}
          </span>
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="truncate font-display text-label font-semibold uppercase text-fg-1">
              {clip.title?.trim() || `Clip ${index + 1}`}
            </span>
            <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
              {formatStreamClock(clip.start_seconds)} → {formatStreamClock(clip.end_seconds)} → Short de{' '}
              {formatStreamClock(clipOutputDuration(clip))}
            </span>
          </span>
        </button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          disabled={disabled}
          onClick={onRemove}
          aria-label={`Quitar corte ${number}`}
        >
          <Trash2 aria-hidden />
        </Button>
      </div>

      <Input
        id={`${clip.id}-title`}
        aria-label={`Título del corte ${number}`}
        value={clip.title ?? ''}
        disabled={disabled}
        onChange={(e) => onClipChange({ title: e.target.value })}
        placeholder={`Clip ${index + 1}`}
        className="h-9"
      />

      {rangeIssue ? (
        <p role="alert" className="text-body-sm text-destructive">
          {rangeIssue}
        </p>
      ) : null}

      <div className="grid grid-cols-2 gap-1.5">
        <Select value={String(speed)} disabled={disabled} onValueChange={(value) => onEditChange({ speed: Number(value) })}>
          <SelectTrigger aria-label="Velocidad de reproducción" className="h-10 w-full gap-1 px-2.5 font-mono tracking-wider">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CLIP_SPEEDS.map((value) => (
              <SelectItem key={value} value={String(value)}>
                {value}×
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled}
          aria-pressed={sourceVolume === 0}
          aria-label={sourceVolume === 0 ? 'Quitar silencio' : 'Silenciar audio original'}
          onClick={toggleMute}
          className={cn(CHIP_CLASS, sourceVolume === 0 && 'border-destructive/45 text-destructive')}
        >
          {sourceVolume === 0 ? 'Silencio' : `Vol ${Math.round(sourceVolume * 100)}%`}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled}
          aria-pressed={fadesOn}
          onClick={() =>
            onEditChange(
              fadesOn
                ? { fade_in_seconds: 0, fade_out_seconds: 0 }
                : { fade_in_seconds: CHIP_FADE_SECONDS, fade_out_seconds: CHIP_FADE_SECONDS },
            )
          }
          className={cn(CHIP_CLASS, fadesOn && 'border-stream/45 text-stream-text')}
        >
          {fadesOn ? `Fundidos ${fadeIn === fadeOut ? fadeIn : `${fadeIn}/${fadeOut}`} s` : 'Sin fundidos'}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled}
          aria-expanded={showText}
          onClick={() => setTextOpen((open) => !open)}
          className={cn(CHIP_CLASS, overlays.length > 0 && 'border-stream/45 text-stream-text')}
        >
          {overlays.length > 0 ? `Texto · ${overlays.length}/${MAX_TEXT_OVERLAYS}` : '+ Texto'}
        </Button>
      </div>

      <Button
        type="button"
        variant="ghost"
        size="sm"
        aria-expanded={rangeOpen}
        onClick={() => setRangeOpen((open) => !open)}
        className="justify-start font-mono uppercase tracking-wider text-fg-3"
      >
        Ajustar rango
        <ChevronDown aria-hidden className={cn('transition-transform duration-(--dur-fast)', rangeOpen && 'rotate-180')} />
      </Button>

      {rangeOpen ? (
        <div className="flex flex-col gap-3 border-t border-border-subtle pt-3">
          <div className="grid grid-cols-2 gap-3">
            <NumberField
              id={`${clip.id}-start`}
              label="Inicio (s)"
              value={clip.start_seconds}
              invalid={rangeIssue !== null}
              disabled={disabled}
              onChange={(value) => onClipChange({ start_seconds: value })}
            />
            <NumberField
              id={`${clip.id}-end`}
              label="Fin (s)"
              value={clip.end_seconds}
              invalid={rangeIssue !== null}
              disabled={disabled}
              onChange={(value) => onClipChange({ end_seconds: value })}
            />
            <NumberField
              id={`${clip.id}-fade-in`}
              label="Fundido entrada (s)"
              value={fadeIn}
              max={5}
              disabled={disabled}
              onChange={(value) => onEditChange({ fade_in_seconds: value })}
            />
            <NumberField
              id={`${clip.id}-fade-out`}
              label="Fundido salida (s)"
              value={fadeOut}
              max={5}
              disabled={disabled}
              onChange={(value) => onEditChange({ fade_out_seconds: value })}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-2">
              <Label htmlFor={`${clip.id}-source-volume`} className="text-label text-fg-2">
                Volumen original
              </Label>
              <output htmlFor={`${clip.id}-source-volume`} className="font-mono text-label tabular-nums text-fg-1">
                {sourceVolume === 0 ? 'Silencio' : `${Math.round(sourceVolume * 100)}%`}
              </output>
            </div>
            <input
              id={`${clip.id}-source-volume`}
              type="range"
              min={0}
              max={2}
              step="0.05"
              value={sourceVolume}
              disabled={disabled}
              aria-label="Volumen del audio original"
              onChange={(e) => onEditChange({ source_volume: Number(e.target.value) })}
              className={STREAM_SLIDER_CLASS}
            />
          </div>
        </div>
      ) : null}

      {showText ? (
        <ClipOverlayEditor clip={clip} disabled={disabled} onEditChange={onEditChange} />
      ) : null}
    </li>
  );
}

function NumberField({
  id,
  label,
  value,
  max,
  invalid,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  max?: number;
  invalid?: boolean;
  disabled: boolean;
  onChange: (value: number) => void;
}): ReactNode {
  // Local text while focused, so clearing the field to retype does not
  // immediately commit 0 into the plan; commit only a valid number, on blur.
  const [text, setText] = useState(String(value));
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    if (!focused) setText(String(value));
  }, [value, focused]);

  const commit = (): void => {
    const parsed = Number(text);
    if (text.trim() !== '' && Number.isFinite(parsed)) {
      onChange(parsed);
    } else {
      setText(String(value));
    }
  };

  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Label htmlFor={id} className="text-label text-fg-2">
        {label}
      </Label>
      <Input
        id={id}
        type="number"
        min={0}
        max={max}
        step="0.1"
        value={text}
        disabled={disabled}
        aria-invalid={invalid}
        onFocus={() => setFocused(true)}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur();
        }}
        onBlur={() => {
          setFocused(false);
          commit();
        }}
        className="h-9 tabular-nums"
      />
    </div>
  );
}

/** Empty input clears an optional numeric field; anything else sets it. */
function optionalNumber(value: string): number | undefined {
  return value === '' ? undefined : Number(value);
}

/** Go's font_size is an int, so typed decimals are rounded before saving. */
function optionalInteger(value: string): number | undefined {
  return value === '' ? undefined : Math.round(Number(value));
}

type OverlayRow = StreamTextOverlay & { rowId: number };

let overlayRowSeq = 0;

function overlayRows(overlays: readonly StreamTextOverlay[]): OverlayRow[] {
  return overlays.map((overlay) => ({ ...overlay, rowId: ++overlayRowSeq }));
}

function committedOverlays(rows: readonly OverlayRow[]): StreamTextOverlay[] {
  return withoutEmptyTextOverlays(rows.map(({ rowId: _rowId, ...overlay }) => overlay));
}

/**
 * The rows are the editable list; only rows with text reach the plan, because
 * the server rejects blank text. A new row, or one whose text was cleared to
 * retype it, stays local (with its timing and position) until it has text.
 */
function ClipOverlayEditor({
  clip,
  disabled,
  onEditChange,
}: {
  clip: StreamClipRange;
  disabled: boolean;
  onEditChange: (patch: Partial<StreamClipEdit>) => void;
}): ReactNode {
  const overlays = clip.edit?.text_overlays ?? [];
  const clipDuration = Math.max(0, clip.end_seconds - clip.start_seconds);
  const [rows, setRows] = useState<OverlayRow[]>(() => overlayRows(overlays));
  const [syncedKey, setSyncedKey] = useState(() => JSON.stringify(overlays));
  const committedKey = JSON.stringify(overlays);
  if (committedKey !== syncedKey && committedKey !== JSON.stringify(committedOverlays(rows))) {
    // The plan changed underneath the rows (a reload, not an edit here): adopt it.
    setRows(overlayRows(overlays));
    setSyncedKey(committedKey);
  }

  const commitRows = (next: OverlayRow[]): void => {
    setRows(next);
    onEditChange({ text_overlays: committedOverlays(next) });
  };
  const addRow = (): void => setRows([...rows, { text: '', position_y: 0.5, rowId: ++overlayRowSeq }]);
  const updateRow = (rowId: number, patch: Partial<StreamTextOverlay>): void =>
    commitRows(rows.map((row) => (row.rowId === rowId ? { ...row, ...patch } : row)));
  const removeRow = (rowId: number): void => commitRows(rows.filter((row) => row.rowId !== rowId));

  return (
    <section className="flex flex-col gap-3 border-t border-border-subtle pt-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
          Textos en pantalla · {rows.length}/{MAX_TEXT_OVERLAYS}
        </span>
        <Button type="button" variant="ghost" size="sm" onClick={addRow} disabled={disabled || rows.length >= MAX_TEXT_OVERLAYS}>
          <Plus aria-hidden />
          Añadir texto
        </Button>
      </div>

      {rows.map((row, index) => (
        <OverlayRow
          key={row.rowId}
          idBase={`${clip.id}-text-${row.rowId}`}
          overlay={row}
          clipDuration={clipDuration}
          disabled={disabled}
          removeLabel={`Eliminar texto ${index + 1}`}
          onChange={(patch) => updateRow(row.rowId, patch)}
          onRemove={() => removeRow(row.rowId)}
        />
      ))}
    </section>
  );
}

function OverlayRow({
  idBase,
  overlay,
  clipDuration,
  disabled,
  removeLabel,
  onChange,
  onRemove,
}: {
  idBase: string;
  overlay: StreamTextOverlay;
  clipDuration: number;
  disabled: boolean;
  removeLabel: string;
  onChange: (patch: Partial<StreamTextOverlay>) => void;
  onRemove: () => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-3 border border-border-subtle bg-surface-1 p-3">
      <div className="flex items-end gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <Label htmlFor={idBase} className="text-label text-fg-2">
            Texto
          </Label>
          <Input
            id={idBase}
            value={overlay.text}
            maxLength={120}
            disabled={disabled}
            aria-invalid={overlay.text.trim() === ''}
            onChange={(e) => onChange({ text: e.target.value })}
            placeholder="NICE SHOT"
            className="h-9"
          />
        </div>
        <Button type="button" variant="ghost" size="icon-sm" disabled={disabled} onClick={onRemove} aria-label={removeLabel}>
          <Trash2 aria-hidden />
        </Button>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="flex min-w-0 flex-col gap-1.5">
          <Label htmlFor={`${idBase}-start`} className="text-label text-fg-2">
            Desde (s)
          </Label>
          <Input
            id={`${idBase}-start`}
            type="number"
            min={0}
            max={clipDuration}
            step="0.1"
            value={overlay.start_seconds ?? ''}
            disabled={disabled}
            onChange={(e) => onChange({ start_seconds: optionalNumber(e.target.value) })}
            placeholder="0"
            className="h-9 tabular-nums"
          />
        </div>
        <div className="flex min-w-0 flex-col gap-1.5">
          <Label htmlFor={`${idBase}-end`} className="text-label text-fg-2">
            Hasta (s)
          </Label>
          <Input
            id={`${idBase}-end`}
            type="number"
            min={0}
            max={clipDuration}
            step="0.1"
            value={overlay.end_seconds ?? ''}
            disabled={disabled}
            onChange={(e) => onChange({ end_seconds: optionalNumber(e.target.value) })}
            placeholder={clipDuration.toFixed(1)}
            className="h-9 tabular-nums"
          />
        </div>
        <div className="flex min-w-0 flex-col gap-1.5">
          <Label htmlFor={`${idBase}-size`} className="text-label text-fg-2">
            Tamaño
          </Label>
          <Input
            id={`${idBase}-size`}
            type="number"
            min={MIN_OVERLAY_FONT_SIZE}
            max={MAX_OVERLAY_FONT_SIZE}
            step="1"
            value={overlay.font_size ?? ''}
            disabled={disabled}
            onChange={(e) => onChange({ font_size: optionalInteger(e.target.value) })}
            placeholder={String(DEFAULT_OVERLAY_FONT_SIZE)}
            className="h-9 tabular-nums"
          />
        </div>
      </div>

      <div className="flex items-center gap-3">
        <Label htmlFor={`${idBase}-position`} className="shrink-0 text-label text-fg-2">
          Posición vertical
        </Label>
        <input
          id={`${idBase}-position`}
          type="range"
          min={STREAMER_BANNER_MIN_POSITION}
          max={STREAMER_BANNER_MAX_POSITION}
          step="0.005"
          value={overlay.position_y}
          disabled={disabled}
          aria-valuetext={`${Math.round(overlay.position_y * 100)}% desde arriba`}
          onChange={(e) => onChange({ position_y: Number(e.target.value) })}
          className={STREAM_SLIDER_CLASS}
        />
        <output
          htmlFor={`${idBase}-position`}
          className="w-11 shrink-0 text-right font-mono text-label tabular-nums text-fg-1"
        >
          {Math.round(overlay.position_y * 100)}%
        </output>
      </div>
    </div>
  );
}
