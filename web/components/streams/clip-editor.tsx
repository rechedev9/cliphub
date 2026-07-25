'use client';

import type { ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import type { StreamClipEdit, StreamClipRange, StreamTextOverlay } from '@/lib/api/streams';
import {
  CLIP_SPEEDS,
  DEFAULT_OVERLAY_FONT_SIZE,
  MAX_OVERLAY_FONT_SIZE,
  MAX_TEXT_OVERLAYS,
  MIN_OVERLAY_FONT_SIZE,
  streamRangeIssue,
} from '@/lib/clip-edit';
import { STREAMER_BANNER_MAX_POSITION, STREAMER_BANNER_MIN_POSITION } from '@/lib/stream-preview';
import { blankClip, pruneClipEdit } from '@/lib/streams/plan';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { STREAM_SLIDER_CLASS } from '@/components/streams/banner-controls';
import { StreamClipTimeline } from '@/components/streams/clip-timeline';
import { cn } from '@/lib/utils';

/** A labelled group of related controls inside a clip card. */
function ControlGroup({
  caption,
  children,
  className,
}: {
  caption: string;
  children: ReactNode;
  className?: string;
}): ReactNode {
  return (
    <div className={cn('flex flex-col gap-2.5 border border-border-subtle bg-surface-1 p-3', className)}>
      <span className="font-mono text-meta uppercase tracking-wider text-fg-3">{caption}</span>
      {children}
    </div>
  );
}

/**
 * Clip ranges: the cut list the whole render is built from.
 *
 * Each clip is one card — identity, its band on the source timeline, then the
 * controls grouped by what they do (range, movement, audio, fades, on-screen
 * text) instead of a single wrapping row of nine anonymous number boxes. The
 * values written to the plan are exactly the ones the previous form wrote.
 */
export function StreamClipEditor({
  clips,
  sourceDuration,
  onChange,
  disabled,
}: {
  clips: StreamClipRange[];
  sourceDuration: number;
  onChange: (clips: StreamClipRange[]) => void;
  disabled: boolean;
}): ReactNode {
  const updateClip = (id: string, patch: Partial<StreamClipRange>) =>
    onChange(clips.map((c) => (c.id === id ? { ...c, ...patch } : c)));
  const removeClip = (id: string) => onChange(clips.filter((c) => c.id !== id));
  const addClip = () => onChange([...clips, blankClip(sourceDuration)]);
  const updateEdit = (id: string, patch: Partial<StreamClipEdit>) =>
    onChange(clips.map((c) => (c.id === id ? { ...c, edit: pruneClipEdit({ ...c.edit, ...patch }) } : c)));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionEyebrow label="RANGOS DE CLIP" count={clips.length} />
        <Button type="button" variant="outline" size="sm" onClick={addClip} disabled={disabled}>
          <Plus className="size-4" aria-hidden />
          AÑADIR
        </Button>
      </div>

      <div className="flex flex-col gap-4">
        {clips.map((clip, i) => {
          const rangeIssue = streamRangeIssue(clip, sourceDuration, i);
          const invalid = rangeIssue !== null;
          return (
            <article
              key={clip.id}
              className={cn(
                '@container/clip flex flex-col gap-4 border bg-surface-2 p-4 shadow-[var(--elev-0)]',
                invalid ? 'border-destructive/50' : 'border-border',
              )}
            >
              <div className="flex flex-wrap items-end gap-3">
                <span
                  aria-hidden
                  className="grid size-11 shrink-0 place-items-center border border-stream/45 bg-stream/10 font-mono text-label tabular-nums text-stream-text"
                >
                  {String(i + 1).padStart(2, '0')}
                </span>
                <div className="flex min-w-40 flex-1 flex-col gap-1.5">
                  <Label htmlFor={`${clip.id}-title`} className="text-label text-fg-2">
                    Título (opcional)
                  </Label>
                  <Input
                    id={`${clip.id}-title`}
                    value={clip.title ?? ''}
                    disabled={disabled}
                    onChange={(e) => updateClip(clip.id, { title: e.target.value })}
                    placeholder={`Clip ${i + 1}`}
                  />
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  disabled={disabled || clips.length <= 1}
                  onClick={() => removeClip(clip.id)}
                  aria-label={`Eliminar Clip ${i + 1}`}
                >
                  <Trash2 className="size-4" aria-hidden />
                </Button>
              </div>

              <StreamClipTimeline clip={clip} sourceDuration={sourceDuration} />

              {rangeIssue ? (
                <p role="alert" className="text-body-sm text-destructive">
                  {rangeIssue}
                </p>
              ) : null}

              <div className="grid gap-3 @[32rem]/clip:grid-cols-2">
                <ControlGroup caption="Rango">
                  <div className="flex flex-wrap items-end gap-3">
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor={`${clip.id}-start`} className="text-label text-fg-2">
                        Inicio (s)
                      </Label>
                      <Input
                        id={`${clip.id}-start`}
                        type="number"
                        min={0}
                        step="0.1"
                        value={clip.start_seconds}
                        disabled={disabled}
                        aria-invalid={invalid}
                        onChange={(e) => updateClip(clip.id, { start_seconds: Number(e.target.value) })}
                        className="w-24 tabular-nums"
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor={`${clip.id}-end`} className="text-label text-fg-2">
                        Fin (s)
                      </Label>
                      <Input
                        id={`${clip.id}-end`}
                        type="number"
                        min={0}
                        step="0.1"
                        value={clip.end_seconds}
                        disabled={disabled}
                        aria-invalid={invalid}
                        onChange={(e) => updateClip(clip.id, { end_seconds: Number(e.target.value) })}
                        className="w-24 tabular-nums"
                      />
                    </div>
                  </div>
                </ControlGroup>

                <ClipMovementGroup clip={clip} disabled={disabled} onEditChange={(patch) => updateEdit(clip.id, patch)} />

                <ClipAudioGroup clip={clip} disabled={disabled} onEditChange={(patch) => updateEdit(clip.id, patch)} />

                <ClipFadeGroup clip={clip} disabled={disabled} onEditChange={(patch) => updateEdit(clip.id, patch)} />
              </div>

              <ClipOverlayEditor
                clip={clip}
                disabled={disabled}
                onEditChange={(patch) => updateEdit(clip.id, patch)}
              />
            </article>
          );
        })}
      </div>
    </div>
  );
}

function ClipMovementGroup({
  clip,
  disabled,
  onEditChange,
}: {
  clip: StreamClipRange;
  disabled: boolean;
  onEditChange: (patch: Partial<StreamClipEdit>) => void;
}): ReactNode {
  const speed = clip.edit?.speed ?? 1;

  return (
    <ControlGroup caption="Movimiento">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${clip.id}-speed`} className="text-label text-fg-2">
          Velocidad
        </Label>
        <Select
          value={String(speed)}
          disabled={disabled}
          onValueChange={(value) => onEditChange({ speed: Number(value) })}
        >
          <SelectTrigger id={`${clip.id}-speed`} aria-label="Velocidad de reproducción" className="w-28">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CLIP_SPEEDS.map((value) => (
              <SelectItem key={value} value={String(value)}>
                {value}x
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </ControlGroup>
  );
}

function ClipAudioGroup({
  clip,
  disabled,
  onEditChange,
}: {
  clip: StreamClipRange;
  disabled: boolean;
  onEditChange: (patch: Partial<StreamClipEdit>) => void;
}): ReactNode {
  const sourceVolume = clip.edit?.source_volume ?? 1;
  const readout = sourceVolume === 0 ? 'Silencio' : `${Math.round(sourceVolume * 100)}%`;

  return (
    <ControlGroup caption="Audio original">
      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2">
          <Label htmlFor={`${clip.id}-source-volume`} className="text-label text-fg-2">
            Volumen original
          </Label>
          <output
            htmlFor={`${clip.id}-source-volume`}
            className="font-mono text-label tabular-nums text-stream-text"
          >
            {readout}
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
          aria-valuetext={readout}
          onChange={(e) => onEditChange({ source_volume: Number(e.target.value) })}
          className={STREAM_SLIDER_CLASS}
        />
      </div>
    </ControlGroup>
  );
}

function ClipFadeGroup({
  clip,
  disabled,
  onEditChange,
}: {
  clip: StreamClipRange;
  disabled: boolean;
  onEditChange: (patch: Partial<StreamClipEdit>) => void;
}): ReactNode {
  return (
    <ControlGroup caption="Fundidos">
      <div className="flex flex-wrap items-end gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor={`${clip.id}-fade-in`} className="text-label text-fg-2">
            Fundido entrada (s)
          </Label>
          <Input
            id={`${clip.id}-fade-in`}
            type="number"
            min={0}
            max={5}
            step="0.1"
            value={clip.edit?.fade_in_seconds ?? 0}
            disabled={disabled}
            onChange={(e) => onEditChange({ fade_in_seconds: Number(e.target.value) })}
            className="w-24 tabular-nums"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor={`${clip.id}-fade-out`} className="text-label text-fg-2">
            Fundido salida (s)
          </Label>
          <Input
            id={`${clip.id}-fade-out`}
            type="number"
            min={0}
            max={5}
            step="0.1"
            value={clip.edit?.fade_out_seconds ?? 0}
            disabled={disabled}
            onChange={(e) => onEditChange({ fade_out_seconds: Number(e.target.value) })}
            className="w-24 tabular-nums"
          />
        </div>
      </div>
    </ControlGroup>
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

  const updateOverlay = (index: number, patch: Partial<StreamTextOverlay>) =>
    onEditChange({ text_overlays: overlays.map((o, i) => (i === index ? { ...o, ...patch } : o)) });
  const removeOverlay = (index: number) =>
    onEditChange({ text_overlays: overlays.filter((_, i) => i !== index) });
  const addOverlay = () => onEditChange({ text_overlays: [...overlays, { text: '', position_y: 0.5 }] });

  return (
    <section className="flex flex-col gap-3 border-t border-border-subtle pt-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
          Textos en pantalla · {overlays.length}/{MAX_TEXT_OVERLAYS}
        </span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={addOverlay}
          disabled={disabled || overlays.length >= MAX_TEXT_OVERLAYS}
        >
          <Plus className="size-4" aria-hidden />
          AÑADIR TEXTO
        </Button>
      </div>

      {overlays.map((overlay, index) => (
        <div key={index} className="flex flex-col gap-3 border border-border-subtle bg-surface-1 p-3">
          <div className="flex items-end gap-3">
            <div className="flex min-w-0 flex-1 flex-col gap-1.5">
              <Label htmlFor={`${clip.id}-text-${index}`} className="text-label text-fg-2">
                Texto
              </Label>
              <Input
                id={`${clip.id}-text-${index}`}
                value={overlay.text}
                maxLength={120}
                disabled={disabled}
                aria-invalid={overlay.text.trim() === ''}
                onChange={(e) => updateOverlay(index, { text: e.target.value })}
                placeholder="NICE SHOT"
              />
            </div>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={disabled}
              onClick={() => removeOverlay(index)}
              aria-label={`Eliminar texto ${index + 1}`}
            >
              <Trash2 className="size-4" aria-hidden />
            </Button>
          </div>

          <div className="flex flex-wrap items-end gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor={`${clip.id}-text-${index}-start`} className="text-label text-fg-2">
                Desde (s)
              </Label>
              <Input
                id={`${clip.id}-text-${index}-start`}
                type="number"
                min={0}
                max={clipDuration}
                step="0.1"
                value={overlay.start_seconds ?? ''}
                disabled={disabled}
                onChange={(e) => updateOverlay(index, { start_seconds: optionalNumber(e.target.value) })}
                placeholder="0"
                className="w-24 tabular-nums"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor={`${clip.id}-text-${index}-end`} className="text-label text-fg-2">
                Hasta (s)
              </Label>
              <Input
                id={`${clip.id}-text-${index}-end`}
                type="number"
                min={0}
                max={clipDuration}
                step="0.1"
                value={overlay.end_seconds ?? ''}
                disabled={disabled}
                onChange={(e) => updateOverlay(index, { end_seconds: optionalNumber(e.target.value) })}
                placeholder={clipDuration.toFixed(1)}
                className="w-24 tabular-nums"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor={`${clip.id}-text-${index}-size`} className="text-label text-fg-2">
                Tamaño
              </Label>
              <Input
                id={`${clip.id}-text-${index}-size`}
                type="number"
                min={MIN_OVERLAY_FONT_SIZE}
                max={MAX_OVERLAY_FONT_SIZE}
                step="1"
                value={overlay.font_size ?? ''}
                disabled={disabled}
                onChange={(e) => updateOverlay(index, { font_size: optionalInteger(e.target.value) })}
                placeholder={String(DEFAULT_OVERLAY_FONT_SIZE)}
                className="w-24 tabular-nums"
              />
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Label
              htmlFor={`${clip.id}-text-${index}-position`}
              className="shrink-0 text-label text-fg-2"
            >
              Posición vertical
            </Label>
            <input
              id={`${clip.id}-text-${index}-position`}
              type="range"
              min={STREAMER_BANNER_MIN_POSITION}
              max={STREAMER_BANNER_MAX_POSITION}
              step="0.005"
              value={overlay.position_y}
              disabled={disabled}
              aria-valuetext={`${Math.round(overlay.position_y * 100)}% desde arriba`}
              onChange={(e) => updateOverlay(index, { position_y: Number(e.target.value) })}
              className={STREAM_SLIDER_CLASS}
            />
            <output
              htmlFor={`${clip.id}-text-${index}-position`}
              className="w-11 shrink-0 text-right font-mono text-label tabular-nums text-stream-text"
            >
              {Math.round(overlay.position_y * 100)}%
            </output>
          </div>
        </div>
      ))}
    </section>
  );
}
