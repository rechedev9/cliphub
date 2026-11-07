'use client';

import type { ReactNode, Ref } from 'react';
import { Music } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { cn } from '@/lib/utils';

/** Bar heights of the decorative waveform, in px: a fixed shape, not the track's audio. */
const WAVE_BARS = Array.from({ length: 40 }, (_, i) => 6 + Math.round(Math.abs(Math.sin(i * 1.7) * 18 + Math.cos(i * 0.6) * 4)));

/** Music volume slider, in UI percent. 100 renders the legacy full-volume form. */
export const MUSIC_VOLUME = { min: 5, max: 100, step: 5, default: 100 } as const;
export const GAME_VOLUME_MIN = 0;

const SLIDER_CLASS =
  'h-1 w-full min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-primary disabled:cursor-not-allowed disabled:opacity-50';

export type MusicCardProps = {
  /** Card heading. */
  title?: string;
  /** The format colour of the flow, for the track tile and the waveform. */
  tone?: FormatTone;
  decided: boolean;
  songTitle: string | null;
  musicVolume: number;
  gameVolume: number;
  busy: boolean;
  /** Lets the footer move focus to the first action while the decision is pending. */
  pickerRef?: Ref<HTMLButtonElement>;
  onOpenPicker: () => void;
  onChooseNone: () => void;
  onClear: () => void;
  onVolumeChange: (volume: number) => void;
  onGameVolumeChange: (volume: number) => void;
};

/** The Short music decision: a track with its mix, explicit "sin música", or still pending. */
export function MusicCard({ title = 'Música', ...props }: MusicCardProps): ReactNode {
  return (
    <section aria-label={title} className="flex flex-col gap-3 rounded-2xl bg-surface-2 p-4">
      <h2 className="font-display text-body font-bold text-fg-1">{title}</h2>
      <MusicDecision {...props} />
    </section>
  );
}

function MusicDecision({
  decided,
  songTitle,
  musicVolume,
  gameVolume,
  busy,
  tone = 'short',
  pickerRef,
  onOpenPicker,
  onChooseNone,
  onClear,
  onVolumeChange,
  onGameVolumeChange,
}: MusicCardProps): ReactNode {
  if (decided && songTitle) {
    return (
      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <span aria-hidden className={cn('grid size-10 shrink-0 place-items-center rounded-xl', FORMAT_TONE[tone].fill)}>
              <Music className="size-4" />
            </span>
            <p className="truncate font-display text-body-sm font-semibold text-fg-1">{songTitle}</p>
          </div>
          <div className="flex shrink-0 gap-1">
            <Button variant="ghost" size="sm" disabled={busy} onClick={onOpenPicker}>
              Cambiar
            </Button>
            <Button variant="ghost" size="sm" disabled={busy} onClick={onClear}>
              Quitar
            </Button>
          </div>
        </div>
        <div aria-hidden className="flex h-7 items-center gap-0.5 overflow-hidden">
          {WAVE_BARS.map((height, index) => (
            <span key={index} style={{ height }} className={cn('w-0.75 shrink-0 rounded-full', FORMAT_TONE[tone].bg)} />
          ))}
        </div>
        <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
          <div className="flex min-w-0 items-center gap-3">
            <label htmlFor="music-volume" className="w-24 shrink-0 text-meta text-fg-2">
              Música <span className="text-fg-1">· {musicVolume}%</span>
            </label>
            <input
              id="music-volume"
              type="range"
              min={MUSIC_VOLUME.min}
              max={MUSIC_VOLUME.max}
              step={MUSIC_VOLUME.step}
              value={musicVolume}
              disabled={busy}
              onChange={(e) => onVolumeChange(Number(e.target.value))}
              className={SLIDER_CLASS}
            />
          </div>
          <div className="flex min-w-0 items-center gap-3">
            <label htmlFor="game-volume" className="w-24 shrink-0 text-meta text-fg-2">
              Juego <span className="text-fg-1">· {gameVolume}%</span>
            </label>
            <input
              id="game-volume"
              type="range"
              min={GAME_VOLUME_MIN}
              max={MUSIC_VOLUME.max}
              step={MUSIC_VOLUME.step}
              value={gameVolume}
              disabled={busy}
              onChange={(e) => onGameVolumeChange(Number(e.target.value))}
              className={SLIDER_CLASS}
            />
          </div>
        </div>
      </div>
    );
  }

  if (decided) {
    // Choosing a track is the only way out of "sin música"; one action, not two lookalikes.
    return (
      <div className="flex flex-col gap-2">
        <p className="font-display text-body-sm font-semibold text-fg-1">Sin música</p>
        <Button type="button" variant="outline" size="sm" disabled={busy} onClick={onOpenPicker} className="self-start">
          <Music className="size-4 shrink-0 text-fg-2" aria-hidden />
          Elegir un tema
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap gap-2">
        <Button
          ref={pickerRef}
          type="button"
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={onOpenPicker}
          className="flex-1"
        >
          <Music className="size-4 shrink-0 text-fg-2" aria-hidden />
          Elegir un tema
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={onChooseNone}
        >
          Sin música
        </Button>
      </div>
    </div>
  );
}
