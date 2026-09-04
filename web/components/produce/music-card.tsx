'use client';

import type { ReactNode } from 'react';
import { Music } from 'lucide-react';
import { Button } from '@/components/ui/button';

/** Music volume slider, in UI percent. 100 renders the legacy full-volume form. */
export const MUSIC_VOLUME = { min: 5, max: 100, step: 5, default: 100 } as const;
export const GAME_VOLUME_MIN = 0;

const SLIDER_CLASS =
  'h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-stream disabled:cursor-not-allowed disabled:opacity-50';

export type MusicCardProps = {
  /** Card eyebrow; the Short constructor numbers it as a step. */
  eyebrow?: string;
  decided: boolean;
  songTitle: string | null;
  musicVolume: number;
  gameVolume: number;
  busy: boolean;
  onOpenPicker: () => void;
  onChooseNone: () => void;
  onClear: () => void;
  onVolumeChange: (volume: number) => void;
  onGameVolumeChange: (volume: number) => void;
};

/** The Short music decision: a track with its mix, explicit "sin música", or still pending. */
export function MusicCard({ eyebrow = 'Música', ...props }: MusicCardProps): ReactNode {
  return (
    <div className="studio-panel flex flex-col gap-2.5 px-3.5 py-3">
      <span className="font-mono text-meta uppercase tracking-ultra text-fg-3">{eyebrow}</span>
      <MusicDecision {...props} />
    </div>
  );
}

function MusicDecision({
  decided,
  songTitle,
  musicVolume,
  gameVolume,
  busy,
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
          <div className="flex min-w-0 items-center gap-2.5">
            <Music className="size-4 shrink-0 text-stream" aria-hidden />
            <p className="truncate font-display text-body-sm font-semibold uppercase text-fg-1">{songTitle}</p>
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
        <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
          <div className="flex items-center gap-3">
            <label htmlFor="music-volume" className="w-24 shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2">
              Música <span className="text-stream-text">· {musicVolume}%</span>
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
          <div className="flex items-center gap-3">
            <label htmlFor="game-volume" className="w-24 shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2">
              Juego <span className="text-stream-text">· {gameVolume}%</span>
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
    return (
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="font-display text-body-sm font-semibold uppercase text-fg-1">Sin música</p>
          <p className="text-body-sm text-fg-3">Solo el audio de la partida.</p>
        </div>
        <div className="flex shrink-0 gap-1">
          <Button variant="ghost" size="sm" disabled={busy} onClick={onOpenPicker}>
            Elegir tema
          </Button>
          <Button variant="ghost" size="sm" disabled={busy} onClick={onClear}>
            Cambiar
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-body-sm text-fg-2">Elige un tema o confirma que va sin música.</p>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={onOpenPicker}
          className="flex min-h-10 flex-1 items-center gap-2.5 border border-dashed border-stream/55 bg-surface-2 px-3 py-2 text-left text-body-sm text-fg-1 transition-colors duration-(--dur-fast) ease-standard hover:border-stream focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Music className="size-4 shrink-0 text-stream" aria-hidden />
          Elegir un tema
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={onChooseNone}
          className="flex min-h-10 items-center border border-border-strong bg-surface-2 px-3 py-2 text-body-sm text-fg-2 transition-colors duration-(--dur-fast) ease-standard hover:border-primary/55 hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50"
        >
          Sin música
        </button>
      </div>
    </div>
  );
}
