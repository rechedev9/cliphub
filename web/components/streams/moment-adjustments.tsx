'use client';

import { useRef, useState, type ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import type { StreamClipEdit, StreamClipRange, StreamEditPlan, StreamTextOverlay } from '@/lib/api/streams';
import {
  CLIP_SPEEDS,
  DEFAULT_OVERLAY_FONT_SIZE,
  MAX_OVERLAY_FONT_SIZE,
  MAX_TEXT_OVERLAYS,
  MIN_OVERLAY_FONT_SIZE,
  withoutEmptyTextOverlays,
} from '@/lib/clip-edit';
import { fadeChoices, shortMoments, textTimingPatch, textWindow, withMomentEdit } from '@/lib/streams/adjustments';
import { STREAMER_BANNER_MAX_POSITION, STREAMER_BANNER_MIN_POSITION } from '@/lib/stream-preview';
import { clipOutputDuration, formatStreamClock } from '@/lib/streams/plan';
import { formatRate } from '@/lib/format';
import { cn } from '@/lib/utils';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

export function ShortAdjustments({
  plan,
  shortId,
  disabled,
  onPlanChange,
}: {
  plan: StreamEditPlan;
  shortId: string;
  disabled: boolean;
  onPlanChange: (plan: StreamEditPlan) => void;
}): ReactNode {
  const moments = shortMoments(plan, shortId);
  if (moments.length === 0) return null;
  const joined = moments.length > 1;
  return (
    <section aria-label="Ajustes del Short" className="flex flex-col gap-3">
      <h3 className="font-semibold">Ajustes del Short</h3>
      <p className="text-body-sm text-fg-2">
        {joined
          ? 'Este Short junta varios momentos. Cada uno tiene sus propios ajustes.'
          : 'Velocidad, volumen, fundidos y textos de este momento.'}
      </p>
      {moments.map(({ clip, name }, index) => (
        <MomentAdjustments
          key={clip.id}
          clip={clip}
          heading={joined ? `${index + 1}. ${name}` : name}
          disabled={disabled}
          onEditChange={(patch) => onPlanChange(withMomentEdit(plan, { clipId: clip.id, patch }))}
        />
      ))}
    </section>
  );
}

function MomentAdjustments({
  clip,
  heading,
  disabled,
  onEditChange,
}: {
  clip: StreamClipRange;
  heading: string;
  disabled: boolean;
  onEditChange: (patch: Partial<StreamClipEdit>) => void;
}): ReactNode {
  const speed = clip.edit?.speed ?? 1;
  const sourceVolume = clip.edit?.source_volume ?? 1;
  const fadeIn = clip.edit?.fade_in_seconds ?? 0;
  const fadeOut = clip.edit?.fade_out_seconds ?? 0;
  const outputSeconds = clipOutputDuration(clip);
  const previousVolumeRef = useRef(1);
  const toggleMute = (): void => {
    if (sourceVolume === 0) {
      onEditChange({ source_volume: previousVolumeRef.current });
      return;
    }
    previousVolumeRef.current = sourceVolume;
    onEditChange({ source_volume: 0 });
  };
  return (
    <article aria-label={heading} className="flex flex-col gap-3 rounded-md border border-border-subtle bg-surface-2 p-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h4 className="truncate font-display text-label font-semibold text-fg-1">{heading}</h4>
        <span className="text-meta tabular-nums text-fg-3">
          {formatStreamClock(clip.start_seconds)} → {formatStreamClock(clip.end_seconds)}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Label htmlFor={`${clip.id}-speed`} className="text-label text-fg-2">
          Velocidad
        </Label>
        <Select value={String(speed)} disabled={disabled} onValueChange={(value) => onEditChange({ speed: Number(value) })}>
          <SelectTrigger id={`${clip.id}-speed`} aria-label="Velocidad de reproducción" className="h-9 w-28 gap-1 px-2.5">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CLIP_SPEEDS.map((value) => (
              <SelectItem key={value} value={String(value)}>
                {formatRate(value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2">
          <Label htmlFor={`${clip.id}-source-volume`} className="text-label text-fg-2">
            Volumen del vídeo
          </Label>
          <output htmlFor={`${clip.id}-source-volume`} className="text-label tabular-nums text-fg-1">
            {sourceVolume === 0 ? 'Silencio' : `${Math.round(sourceVolume * 100)} %`}
          </output>
        </div>
        <div className="flex items-center gap-3">
          <input
            id={`${clip.id}-source-volume`}
            type="range"
            min={0}
            max={2}
            step="0.05"
            value={sourceVolume}
            disabled={disabled}
            onChange={(e) => onEditChange({ source_volume: Number(e.target.value) })}
            className={STREAM_SLIDER_CLASS}
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            aria-pressed={sourceVolume === 0}
            onClick={toggleMute}
            className={cn(sourceVolume === 0 && 'border-destructive/45 text-destructive')}
          >
            {sourceVolume === 0 ? 'Quitar silencio' : 'Silenciar'}
          </Button>
        </div>
      </div>

      <FadePicker
        label="Fundido al empezar"
        current={fadeIn}
        other={fadeOut}
        outputSeconds={outputSeconds}
        disabled={disabled}
        onChange={(seconds) => onEditChange({ fade_in_seconds: seconds })}
      />
      <FadePicker
        label="Fundido al terminar"
        current={fadeOut}
        other={fadeIn}
        outputSeconds={outputSeconds}
        disabled={disabled}
        onChange={(seconds) => onEditChange({ fade_out_seconds: seconds })}
      />

      <TextOverlayEditor clip={clip} disabled={disabled} onEditChange={onEditChange} />
    </article>
  );
}

function FadePicker({
  label,
  current,
  other,
  outputSeconds,
  disabled,
  onChange,
}: {
  label: string;
  current: number;
  other: number;
  outputSeconds: number;
  disabled: boolean;
  onChange: (seconds: number) => void;
}): ReactNode {
  const choices = fadeChoices({ current, other, outputSeconds });
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-label text-fg-2">{label}</span>
      <Segmented
        label={label}
        value={String(current)}
        options={choices.map((choice) => ({
          value: String(choice.seconds),
          label: choice.label,
          disabled: disabled || (!choice.fits && choice.seconds !== current),
          reason: choice.fits ? undefined : 'No cabe en este momento',
        }))}
        onChange={(value) => onChange(Number(value))}
      />
    </div>
  );
}

type OverlayRow = StreamTextOverlay & { rowId: number };

let overlayRowSeq = 0;

function overlayRows(overlays: readonly StreamTextOverlay[]): OverlayRow[] {
  return overlays.map((overlay) => ({ ...overlay, rowId: ++overlayRowSeq }));
}

function committedOverlays(rows: readonly OverlayRow[]): StreamTextOverlay[] {
  return withoutEmptyTextOverlays(rows.map(({ rowId: _rowId, ...overlay }) => overlay));
}

function optionalInteger(value: string): number | undefined {
  return value === '' ? undefined : Math.round(Number(value));
}

function TextOverlayEditor({
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
    <section aria-label="Textos en pantalla" className="flex flex-col gap-3 border-t border-border-subtle pt-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-label text-fg-2">
          Textos en pantalla · {rows.length}/{MAX_TEXT_OVERLAYS}
        </span>
        <Button type="button" variant="ghost" size="sm" onClick={addRow} disabled={disabled || rows.length >= MAX_TEXT_OVERLAYS}>
          <Plus aria-hidden />
          Añadir texto
        </Button>
      </div>
      {rows.map((row, index) => (
        <OverlayRowEditor
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

function OverlayRowEditor({
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
  const timing = textWindow(overlay, clipDuration);
  const drag = (edge: 'start' | 'end', seconds: number) => onChange(textTimingPatch({ overlay, edge, seconds, clipDuration }));
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
        <div className="flex w-20 shrink-0 flex-col gap-1.5">
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
        <Button type="button" variant="ghost" size="icon-sm" disabled={disabled} onClick={onRemove} aria-label={removeLabel}>
          <Trash2 aria-hidden />
        </Button>
      </div>
      <TimingSlider
        id={`${idBase}-start`}
        label="Aparece"
        seconds={timing.start}
        max={clipDuration}
        disabled={disabled}
        onChange={(seconds) => drag('start', seconds)}
      />
      <TimingSlider
        id={`${idBase}-end`}
        label="Desaparece"
        seconds={timing.end}
        max={clipDuration}
        disabled={disabled}
        onChange={(seconds) => drag('end', seconds)}
      />
      <div className="flex items-center gap-3">
        <Label htmlFor={`${idBase}-position`} className="w-24 shrink-0 text-label text-fg-2">
          Altura
        </Label>
        <input
          id={`${idBase}-position`}
          type="range"
          min={STREAMER_BANNER_MIN_POSITION}
          max={STREAMER_BANNER_MAX_POSITION}
          step="0.005"
          value={overlay.position_y}
          disabled={disabled}
          aria-valuetext={`${Math.round(overlay.position_y * 100)} % desde arriba`}
          onChange={(e) => onChange({ position_y: Number(e.target.value) })}
          className={STREAM_SLIDER_CLASS}
        />
        <output htmlFor={`${idBase}-position`} className="w-11 shrink-0 text-right text-label tabular-nums text-fg-1">
          {Math.round(overlay.position_y * 100)} %
        </output>
      </div>
    </div>
  );
}

function TimingSlider({
  id,
  label,
  seconds,
  max,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  seconds: number;
  max: number;
  disabled: boolean;
  onChange: (seconds: number) => void;
}): ReactNode {
  return (
    <div className="flex items-center gap-3">
      <Label htmlFor={id} className="w-24 shrink-0 text-label text-fg-2">
        {label}
      </Label>
      <input
        id={id}
        type="range"
        min={0}
        max={max}
        step="0.1"
        value={seconds}
        disabled={disabled}
        aria-valuetext={formatStreamClock(seconds)}
        onChange={(e) => onChange(Number(e.target.value))}
        className={STREAM_SLIDER_CLASS}
      />
      <output htmlFor={id} className="w-11 shrink-0 text-right text-label tabular-nums text-fg-1">
        {formatStreamClock(seconds)}
      </output>
    </div>
  );
}
