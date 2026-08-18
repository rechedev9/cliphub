'use client';

import { useEffect, useRef, useState, type ReactElement } from 'react';
import { MediaFrame, type MediaFrameAspect } from '@/components/studio/media-frame';
import { editorApi } from '@/lib/api/editor';
import {
  evaluateTimeline,
  itemSpeed,
  itemTimelineEnd,
  type EditorDocument,
  type EditorItem,
} from '@/lib/editor/evaluate';
import { clampVolumeForPreview, itemPlaybackRate, upcomingItems } from '@/lib/editor/playback';

const SEEK_SLOP = 0.12;
const DEFAULT_MUSIC_VOLUME = 0.25;
const PRELOAD_HORIZON = 1;

export type EditorPreviewPlayerProps = {
  doc: EditorDocument;
  time: number;
  playing: boolean;
  onTime: (t: number) => void;
  onEnded: () => void;
};

export function EditorPreviewPlayer({ doc, time, playing, onEnded }: EditorPreviewPlayerProps): ReactElement {
  const sample = evaluateTimeline(doc, time);
  const visibleIds = new Set(sample.layers.map((layer) => layer.item_id));
  const hiddenItems = upcomingItems(doc, time, PRELOAD_HORIZON).filter((item) => !visibleIds.has(item.id));
  const nodes = useRef(new Map<string, HTMLVideoElement>());
  const musicRef = useRef<HTMLAudioElement | null>(null);
  const [failedMusic, setFailedMusic] = useState<string | null>(null);
  const musicKey = doc.music?.key;
  const musicSrc = musicKey !== undefined && musicKey !== '' && musicKey !== failedMusic ? `/api/songs/${musicKey}/audio` : null;

  useEffect(() => {
    for (const [id, node] of nodes.current) {
      const item = findItem(doc, id);
      if (item === undefined) continue;
      syncVideo(node, item, sourceTimeAt(item, time), playing && itemInRange(item, time));
    }
  }, [doc, time, playing]);

  useEffect(() => {
    const node = musicRef.current;
    if (node === null) return;
    node.volume = doc.music?.volume ?? DEFAULT_MUSIC_VOLUME;
    if (playing) {
      void node.play().catch(() => undefined);
    } else {
      node.pause();
    }
  }, [doc.music?.volume, playing, musicSrc]);

  useEffect(() => {
    if (!playing) return;
    const duration = sample.duration;
    if (duration > 0 && time >= duration) onEnded();
  }, [playing, time, sample.duration, onEnded]);

  return (
    <MediaFrame
      aspect={canvasAspect(doc.canvas)}
      className="mx-auto max-h-[52vh] bg-black"
      media={
        <div className="absolute inset-0">
          {sample.layers.map((layer) => {
            const item = findItem(doc, layer.item_id);
            if (item === undefined) return null;
            return (
              <video
                key={layer.item_id}
                ref={(node) => bindVideo(nodes.current, layer.item_id, node, item, layer.source_time, playing)}
                src={editorApi.assetMediaUrl(layer.asset_id)}
                className="absolute"
                style={{
                  left: `${layer.transform.x * 100}%`,
                  top: `${layer.transform.y * 100}%`,
                  width: `${layer.transform.width * 100}%`,
                  height: `${layer.transform.height * 100}%`,
                  opacity: layer.opacity,
                  objectFit: 'cover',
                }}
                playsInline
                preload="auto"
              />
            );
          })}
          {hiddenItems.map((item) => (
            <video
              key={`preload-${item.id}`}
              ref={(node) =>
                bindVideo(nodes.current, item.id, node, item, sourceTimeAt(item, time), playing && itemInRange(item, time))
              }
              src={editorApi.assetMediaUrl(item.asset_id)}
              className="hidden"
              playsInline
              preload="auto"
            />
          ))}
          {sample.texts.map((text) => (
            <div
              key={text.id}
              className="pointer-events-none absolute right-0 left-0 text-center font-bold text-white"
              style={{ top: `${text.position_y * 100}%`, fontSize: text.font_size, transform: 'translateY(-50%)' }}
            >
              {text.text}
            </div>
          ))}
          {musicSrc !== null ? (
            <audio
              ref={musicRef}
              loop
              src={musicSrc}
              onError={() => {
                if (musicKey !== undefined) setFailedMusic(musicKey);
              }}
            />
          ) : null}
        </div>
      }
    />
  );
}

function canvasAspect(canvas: EditorDocument['canvas']): MediaFrameAspect {
  return canvas.width > canvas.height ? '16:9' : '9:16';
}

function findItem(doc: EditorDocument, id: string): EditorItem | undefined {
  for (const track of doc.tracks) {
    for (const item of track.items) {
      if (item.id === id) return item;
    }
  }
  return undefined;
}

function itemInRange(item: EditorItem, time: number): boolean {
  return time >= item.timeline_start && time < itemTimelineEnd(item);
}

function sourceTimeAt(item: EditorItem, time: number): number {
  if (!itemInRange(item, time)) return item.source_in;
  return item.source_in + (time - item.timeline_start) * itemSpeed(item);
}

function bindVideo(
  nodes: Map<string, HTMLVideoElement>,
  id: string,
  node: HTMLVideoElement | null,
  item: EditorItem,
  sourceTime: number,
  shouldPlay: boolean,
): void {
  if (node === null) {
    nodes.delete(id);
    return;
  }
  nodes.set(id, node);
  syncVideo(node, item, sourceTime, shouldPlay);
}

function syncVideo(node: HTMLVideoElement, item: EditorItem, sourceTime: number, shouldPlay: boolean): void {
  const rate = itemPlaybackRate(item);
  if (node.playbackRate !== rate) node.playbackRate = rate;
  const volume = clampVolumeForPreview(item.volume);
  if (node.volume !== volume) node.volume = volume;
  if (node.muted) node.muted = false;
  if (Math.abs(node.currentTime - sourceTime) > SEEK_SLOP) node.currentTime = sourceTime;
  if (shouldPlay) {
    if (node.paused) void node.play().catch(() => undefined);
    return;
  }
  if (!node.paused) node.pause();
}
