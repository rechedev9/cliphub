'use client';

import { useEffect, useRef, useState } from 'react';
import { Music } from 'lucide-react';
import { api } from '@/lib/api';
import type { Preset, Song, Video } from '@/lib/api/types';
import { DEFAULT_EDIT_CONFIG } from '@/lib/api/reel-store';
import {
  MUSIC_VOLUME_DEFAULT_PERCENT,
  MUSIC_VOLUME_MAX_PERCENT,
  MUSIC_VOLUME_MIN_PERCENT,
  MUSIC_VOLUME_STEP_PERCENT,
  musicChoicesEqual,
  musicVolumePercentToRequest,
  musicVolumeRequestToPercent,
  type MusicChoice,
} from '@/lib/api/reel-music';
import { canRerenderWithMusic, reelCreativeBrief } from '@/lib/reel-brief';
import { SongCatalog } from '@/components/clips/song-picker-dialog';
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
  const [preset, setPreset] = useState<Preset | null>(null);
  const [briefApproved, setBriefApproved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wasOpen = useRef(false);

  useEffect(() => {
    if (open && !wasOpen.current) {
      setSong(video.songId ? { id: video.songId, title: video.songId, artist: '', genre: '', previewUrl: '', durationSec: 0 } : null);
      setVolumePercent(musicVolumeRequestToPercent(video.musicVolume));
      setBriefApproved(false);
      setError(null);
    }
    wasOpen.current = open;
  }, [open, video.songId, video.musicVolume]);

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

  useEffect(() => {
    setBriefApproved(false);
  }, [song?.id, volumePercent]);

  const draft: MusicChoice = song
    ? { songId: song.id, musicVolume: musicVolumePercentToRequest(volumePercent) }
    : {};
  const musicChanged = !musicChoicesEqual(original, draft);
  const briefItems = reelCreativeBrief(
    video.editConfig ?? DEFAULT_EDIT_CONFIG,
    preset,
    song?.title ?? null,
    song ? volumePercent : MUSIC_VOLUME_DEFAULT_PERCENT,
  );
  const ready = canRerenderWithMusic({ briefApproved, busy, musicChanged });

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
          <div className="flex items-center gap-4 border border-border px-4 py-3">
            <label
              htmlFor={`library-music-volume-${video.id}`}
              className="shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2"
            >
              VOLUMEN <span className="text-stream-text">· {volumePercent}%</span>
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
            Brief creativo exacto
          </p>
          <dl className="mt-2.5 grid gap-x-6 gap-y-1.5 text-body-sm">
            {briefItems.map((item) => (
              <div key={item.label} className="flex min-w-0 gap-1.5">
                <dt className="shrink-0 text-fg-3">{item.label}:</dt>
                <dd className="truncate text-fg-1" title={item.value}>{item.value}</dd>
              </div>
            ))}
          </dl>
          <label className="mt-3.5 flex min-h-10 items-center gap-2.5 text-body-sm text-fg-1">
            <input
              type="checkbox"
              checked={briefApproved}
              disabled={!musicChanged || busy}
              onChange={(event) => setBriefApproved(event.target.checked)}
              className="size-5 shrink-0 cursor-pointer accent-primary disabled:cursor-not-allowed disabled:opacity-50"
            />
            Apruebo este brief exacto para el nuevo render.
          </label>
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
    ? { songId: video.songId, musicVolume: video.musicVolume }
    : {};
}
