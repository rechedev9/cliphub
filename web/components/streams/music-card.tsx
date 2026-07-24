'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Loader2, Music4, Pause, Play, Sparkles } from 'lucide-react';
import { api } from '@/lib/api';
import type { Song } from '@/lib/api/types';
import { MUSIC_VOLUMES, NO_MUSIC_VALUE } from '@/lib/streams/plan';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

/**
 * Background music and the viral grade. The music bed is mixed under the
 * streamer's own audio at render time, so the gain presets here are the same
 * three the plan stores — the slider-free choice is deliberate.
 */
export function StreamMusicCard({
  musicKey,
  volume,
  grade,
  busy,
  onMusicKey,
  onMusicVolume,
  onGrade,
}: {
  musicKey: string;
  volume: number;
  grade: boolean;
  busy: boolean;
  onMusicKey: (key: string) => void;
  onMusicVolume: (volume: number) => void;
  onGrade: (grade: boolean) => void;
}): ReactNode {
  const [songs, setSongs] = useState<Song[] | null>(null);
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
  const previewRequest = useRef(0);

  useEffect(() => {
    let active = true;
    api
      .listSongs()
      .then((next) => {
        if (active) setSongs(next);
      })
      .catch(() => {
        if (active) setSongs([]);
      });
    return () => {
      active = false;
    };
  }, []);

  const selectedSong = songs?.find((song) => song.id === musicKey);

  const stopAndResetPreview = useCallback(() => {
    previewRequest.current += 1;
    const audio = audioRef.current;
    if (audio) {
      audio.pause();
      audio.currentTime = 0;
    }
    setPreviewPlaying(false);
  }, []);

  useEffect(() => {
    stopAndResetPreview();
    setPreviewError(null);
  }, [musicKey, busy, stopAndResetPreview]);

  useEffect(() => {
    const audio = audioRef.current;
    return () => {
      previewRequest.current += 1;
      if (audio) {
        audio.pause();
        audio.currentTime = 0;
      }
    };
  }, []);

  const togglePreview = async (): Promise<void> => {
    const audio = audioRef.current;
    if (!audio || !selectedSong?.previewUrl || busy || songs === null) return;

    if (previewPlaying) {
      previewRequest.current += 1;
      audio.pause();
      setPreviewPlaying(false);
      return;
    }

    const request = ++previewRequest.current;
    setPreviewError(null);
    try {
      await audio.play();
      if (previewRequest.current === request) setPreviewPlaying(true);
    } catch {
      if (previewRequest.current !== request) return;
      audio.pause();
      audio.currentTime = 0;
      setPreviewPlaying(false);
      setPreviewError('No se pudo reproducir la vista previa de esta canción.');
    }
  };

  let selectedMusicLabel = 'Ninguna';
  if (songs === null) {
    selectedMusicLabel = musicKey ? 'Cargando pista…' : 'Cargando pistas…';
  } else if (selectedSong) {
    selectedMusicLabel = `${selectedSong.title}${selectedSong.genre ? ` · ${selectedSong.genre}` : ''}`;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionEyebrow label="MÚSICA Y EFECTOS" />
        {musicKey ? (
          <StatusTag tone="stream" icon={Music4}>
            {Math.round(volume * 100)}%
          </StatusTag>
        ) : (
          <StatusTag>SIN MÚSICA</StatusTag>
        )}
      </div>

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex min-w-0 flex-col gap-1.5">
          <Label htmlFor="stream-music" className="text-label text-fg-2">
            Música de fondo
          </Label>
          <div className="flex items-center gap-2">
            <Select
              value={musicKey || NO_MUSIC_VALUE}
              disabled={busy || songs === null}
              onValueChange={(value) => onMusicKey(value === NO_MUSIC_VALUE ? '' : value)}
            >
              <SelectTrigger id="stream-music" className="w-72 max-w-[calc(80vw-2.5rem)]">
                <SelectValue>{selectedMusicLabel}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NO_MUSIC_VALUE}>Ninguna</SelectItem>
                {(songs ?? []).map((song) => (
                  <SelectItem key={song.id} value={song.id}>
                    {song.title}
                    {song.genre ? ` · ${song.genre}` : ''}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="outline"
              size="icon"
              disabled={busy || songs === null || !selectedSong?.previewUrl}
              onClick={() => void togglePreview()}
              aria-label={`${previewPlaying ? 'Pausar' : 'Escuchar'} ${selectedSong?.title ?? 'música seleccionada'}`}
              className="shrink-0"
            >
              {previewPlaying ? <Pause className="size-4" /> : <Play className="size-4" />}
            </Button>
          </div>
          <audio
            ref={audioRef}
            src={selectedSong?.previewUrl}
            preload="none"
            data-music-preview
            className="hidden"
            onPlay={() => setPreviewPlaying(true)}
            onPause={() => setPreviewPlaying(false)}
            onEnded={stopAndResetPreview}
            onError={() => {
              stopAndResetPreview();
              setPreviewError('No se pudo reproducir la vista previa de esta canción.');
            }}
          />
          {songs === null ? (
            <p role="status" className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
              <Loader2 aria-hidden className="size-3.5 animate-spin" />
              Cargando catálogo local
            </p>
          ) : null}
          {songs !== null && songs.length === 0 ? (
            <p className="text-body-sm text-fg-3">
              No hay pistas en el catálogo local. El render seguirá adelante sin música.
            </p>
          ) : null}
          {previewError ? (
            <p role="alert" className="text-body-sm text-destructive">
              {previewError}
            </p>
          ) : null}
        </div>

        {musicKey ? (
          <div className="flex flex-col gap-1.5">
            <Label className="text-label text-fg-2">Volumen de música</Label>
            <ToggleGroup
              type="single"
              variant="filter"
              spacing={2}
              value={String(volume)}
              onValueChange={(v) => v && onMusicVolume(Number(v))}
              disabled={busy}
              aria-label="Volumen de música"
            >
              {MUSIC_VOLUMES.map((v) => (
                <ToggleGroupItem key={v.value} value={String(v.value)}>
                  {v.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Button
          type="button"
          variant={grade ? 'default' : 'outline'}
          size="sm"
          disabled={busy}
          aria-pressed={grade}
          onClick={() => onGrade(!grade)}
        >
          <Sparkles className="size-4" aria-hidden />
          {grade ? 'Gradación viral: activada' : 'Gradación viral: desactivada'}
        </Button>
        <p className="min-w-56 flex-1 text-body-sm text-fg-3">
          Ligero realce de contraste y saturación. La música se mezcla bajo el audio del streamer.
        </p>
      </div>
    </div>
  );
}
