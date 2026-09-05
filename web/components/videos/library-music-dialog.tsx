'use client';

import { useEffect, useRef, useState } from 'react';
import { Music } from 'lucide-react';
import { api } from '@/lib/api';
import type { Preset, Song, Video } from '@/lib/api/types';
import { DEFAULT_EDIT_CONFIG } from '@/lib/api/reel-store';
import {
  GAME_VOLUME_DEFAULT_PERCENT,
  GAME_VOLUME_MAX_PERCENT,
  GAME_VOLUME_MIN_PERCENT,
  GAME_VOLUME_STEP_PERCENT,
  MUSIC_VOLUME_DEFAULT_PERCENT,
  MUSIC_VOLUME_MAX_PERCENT,
  MUSIC_VOLUME_MIN_PERCENT,
  MUSIC_VOLUME_STEP_PERCENT,
  gameVolumePercentToRequest,
  gameVolumeRequestToPercent,
  musicChoicesEqual,
  musicVolumePercentToRequest,
  musicVolumeRequestToPercent,
  type MusicChoice,
} from '@/lib/api/reel-music';
import { canRerenderWithMusic, reelCreativeBrief } from '@/lib/reel-brief';
import { SongCatalog } from '@/components/clips/song-picker-dialog';
import { CreativeBriefList } from '@/components/studio/creative-brief';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

export function LibraryMusicDialog({
  open,
  video,
  onOpenChange,
  onApplied,
}: {
  open: boolean;
  video: Video;
  onOpenChange: (open: boolean) => void;
  onApplied: () => void;
}) {
  const original = currentMusicChoice(video);
  const [song, setSong] = useState<Song | null>(null);
  const [volumePercent, setVolumePercent] = useState(MUSIC_VOLUME_DEFAULT_PERCENT);
  const [gameVolumePercent, setGameVolumePercent] = useState(GAME_VOLUME_DEFAULT_PERCENT);
  const [preset, setPreset] = useState<Preset | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wasOpen = useRef(false);

  useEffect(() => {
    if (open && !wasOpen.current) {
      setSong(video.songId ? { id: video.songId, title: video.songId, artist: '', genre: '', previewUrl: '', durationSec: 0 } : null);
      setVolumePercent(musicVolumeRequestToPercent(video.musicVolume));
      setGameVolumePercent(gameVolumeRequestToPercent(video.gameVolume));

      setError(null);
    }
    wasOpen.current = open;
  }, [open, video.songId, video.musicVolume, video.gameVolume]);

  useEffect(() => {
    if (!open || !video.songId) return;
    let active = true;
    void api.listSongs().then((songs) => {
      if (!active) return;
      const match = songs.find((item) => item.id === video.songId);
      if (match) {
        setSong((current) => (current?.id === match.id ? match : current));
      }
    });
    return () => {
      active = false;
    };
  }, [open, video.songId]);

  useEffect(() => {
    if (!open) return;
    let active = true;
    void api.listPresets().then((presets) => {
      if (!active) return;
      setPreset(presets.find((item) => item.name === video.variant) ?? presets.find((item) => item.default) ?? presets[0] ?? null);
    });
    return () => {
      active = false;
    };
  }, [open, video.variant]);

  const draft: MusicChoice = song
    ? {
        songId: song.id,
        musicVolume: musicVolumePercentToRequest(volumePercent),
        gameVolume: gameVolumePercentToRequest(gameVolumePercent),
      }
    : {};
  const musicChanged = !musicChoicesEqual(original, draft);
  const briefItems = reelCreativeBrief(
    video.editConfig ?? DEFAULT_EDIT_CONFIG,
    preset,
    song
      ? { status: 'track', title: song.title, volumePercent, gameVolumePercent }
      : { status: 'none' },
  );
  const ready = canRerenderWithMusic({ busy, musicChanged });

  async function apply(): Promise<void> {
    if (!ready) return;
    setBusy(true);
    setError(null);
    try {
      await api.rerenderVideoMusic(video.id, draft);
      onOpenChange(false);
      onApplied();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo volver a renderizar con música.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{video.songId ? 'Cambiar música' : 'Añadir música'}</DialogTitle>
          <DialogDescription>
            La captura no se repite. El montaje se vuelve a renderizar y sustituye el MP4 actual.
          </DialogDescription>
        </DialogHeader>

        <SongCatalog
          active={open}
          selectedSongId={song?.id ?? null}
          onChoose={(next) => setSong(next)}
        />

        {song ? (
          <div className="flex flex-col gap-3 border border-border px-4 py-3">
            <div className="flex items-center gap-4">
              <label
                htmlFor={`library-music-volume-${video.id}`}
                className="w-36 shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2"
              >
                MÚSICA <span className="text-stream-text">· {volumePercent}%</span>
              </label>
              <input
                id={`library-music-volume-${video.id}`}
                type="range"
                min={MUSIC_VOLUME_MIN_PERCENT}
                max={MUSIC_VOLUME_MAX_PERCENT}
                step={MUSIC_VOLUME_STEP_PERCENT}
                value={volumePercent}
                disabled={busy}
                onChange={(event) => setVolumePercent(Number(event.target.value))}
                className="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-stream disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
            <div className="flex items-center gap-4">
              <label
                htmlFor={`library-game-volume-${video.id}`}
                className="w-36 shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2"
              >
                JUEGO <span className="text-stream-text">· {gameVolumePercent}%</span>
              </label>
              <input
                id={`library-game-volume-${video.id}`}
                type="range"
                min={GAME_VOLUME_MIN_PERCENT}
                max={GAME_VOLUME_MAX_PERCENT}
                step={GAME_VOLUME_STEP_PERCENT}
                value={gameVolumePercent}
                disabled={busy}
                onChange={(event) => setGameVolumePercent(Number(event.target.value))}
                className="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-stream disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
          </div>
        ) : null}

        {video.songId || song ? (
          <Button
            type="button"
            variant="ghost"
            className="self-start"
            disabled={busy || !song}
            onClick={() => {
              setSong(null);
              setVolumePercent(MUSIC_VOLUME_DEFAULT_PERCENT);
              setGameVolumePercent(GAME_VOLUME_DEFAULT_PERCENT);
            }}
          >
            Quitar música
          </Button>
        ) : null}

        <section className="studio-panel px-4 py-3" aria-labelledby={`library-music-brief-${video.id}`}>
          <p
            id={`library-music-brief-${video.id}`}
            className="font-mono text-meta uppercase tracking-wider text-primary"
          >
            Configuración del render
          </p>
          <CreativeBriefList items={briefItems} className="mt-2.5" />
        </section>

        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}

        <Button
          type="button"
          variant="hero"
          className="w-full"
          disabled={!ready}
          loading={busy}
          loadingText="RENDERIZANDO DE NUEVO…"
          onClick={() => void apply()}
        >
          <Music className="size-4" aria-hidden /> VOLVER A RENDERIZAR
        </Button>
      </DialogContent>
    </Dialog>
  );
}

function currentMusicChoice(video: Video): MusicChoice {
  return video.songId
    ? { songId: video.songId, musicVolume: video.musicVolume, gameVolume: video.gameVolume }
    : {};
}
