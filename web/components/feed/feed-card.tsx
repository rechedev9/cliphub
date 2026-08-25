'use client';

import { useState } from 'react';
import { Heart, Play } from 'lucide-react';
import type { FeedItem } from '@/lib/api/types';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { MediaFrame } from '@/components/studio/media-frame';
import { TiltSurface } from '@/components/studio/tilt-surface';
import { timeAgo } from '@/lib/format';
import { cn } from '@/lib/utils';

export type FeedCardProps = {
  item: FeedItem;
  /** 1-based rank while a sort is active, so TOP SEMANA visibly reorders cards. */
  rank?: number;
};

/** One community reel. Play is the dominant control; likes stay magenta. */
export function FeedCard({ item, rank }: FeedCardProps) {
  const [liked, setLiked] = useState(false);
  const [playerOpen, setPlayerOpen] = useState(false);
  const likeCount = item.likes + (liked ? 1 : 0);
  const initials = item.author.slice(0, 2).toUpperCase();

  return (
    <figure className="studio-panel studio-panel-interactive studio-defer-render flex flex-col overflow-hidden">
      <MediaFrame
        // `aspect-[4/5]` wins the cn() merge over MediaFrame's own aspect class.
        className="studio-rim aspect-[4/5] border-b border-border"
        badge={
          rank !== undefined ? (
            <span className="inline-flex min-h-7 items-center gap-1 border border-primary/55 bg-surface-0/90 px-2 font-mono text-meta tabular-nums text-primary shadow-[var(--glow-primary-sm)]">
              <span aria-hidden>#</span>
              <span className="sr-only">Puesto </span>
              {rank}
            </span>
          ) : undefined
        }
        footer={
          <span className="inline-flex min-h-7 items-center border border-border-strong bg-surface-0/85 px-2.5 font-mono text-meta uppercase text-fg-1">
            {item.map}
          </span>
        }
        media={
          <TiltSurface className="size-full" planeClassName="size-full">
            {/* The generated plate sits underneath so a cover the CSP blocks —
                `img-src 'self' data: blob:` rules out any off-origin thumbnail —
                degrades to brand art instead of a stretched broken-image glyph. */}
            <div className="absolute inset-0 scale-[1.06] transition-transform duration-(--dur-base) ease-standard group-hover/frame:scale-[1.1]">
              <ReelCover seed={item.id} plain className="absolute inset-0" />
              <CoverImage src={item.thumbnailUrl} className="absolute inset-0" />
            </div>

            {/*
              The play control rides the tilt plane rather than floating over it,
              so it parallaxes against the cover instead of sliding with it: at
              `translateZ(22px)` inside the plane's `preserve-3d` space it is a
              real object resting on the picture, casting `--elev-4` onto it. It
              also has to live inside the plane for pointermove to reach the tilt
              listener at all — a sibling overlay would swallow every event and
              kill both the tilt and the cover's hover scale.
            */}
            <button
              type="button"
              onClick={() => setPlayerOpen(true)}
              aria-label={`Reproducir ${item.title}`}
              className="group/play absolute inset-0 flex items-center justify-center outline-none [transform:translateZ(22px)] focus-visible:outline-2 focus-visible:-outline-offset-4 focus-visible:outline-ring"
            >
              <span className="relative grid size-16 place-items-center rounded-full border-2 border-primary/60 bg-surface-2/90 text-primary shadow-[var(--elev-4)] transition-transform duration-(--dur-base) ease-pop group-hover/play:scale-110">
                {/* The bloom is its own layer so the disc keeps its cast shadow;
                    a token glow used alone degrades to `none` cleanly. */}
                <span
                  aria-hidden
                  className="absolute inset-0 rounded-full opacity-0 transition-opacity duration-(--dur-base) ease-standard [box-shadow:var(--glow-primary-lg)] group-hover/play:opacity-100"
                />
                <Play className="relative ml-1 size-7 fill-current" aria-hidden />
              </span>
            </button>
          </TiltSurface>
        }
      />

      <figcaption className="flex flex-1 flex-col gap-3.5 p-4">
        <div className="min-w-0">
          <h3 className="line-clamp-2 font-display text-body-lg font-bold leading-snug text-fg-1">
            {item.title}
          </h3>
          <p className="mt-1.5 font-mono text-meta uppercase text-fg-3">{timeAgo(item.createdAt)}</p>
        </div>

        <div className="mt-auto flex items-center justify-between gap-3 border-t border-border pt-3">
          <div className="flex min-w-0 items-center gap-2.5">
            <Avatar className="size-8 rounded-md border border-border-strong">
              <AvatarImage src={item.authorAvatarUrl} alt={item.author} />
              <AvatarFallback className="rounded-md text-meta">{initials}</AvatarFallback>
            </Avatar>
            <span className="min-w-0 truncate font-mono text-meta text-fg-2">@{item.author}</span>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setLiked((v) => !v)}
            aria-pressed={liked}
            aria-label={liked ? 'Quitar me gusta' : 'Me gusta'}
            className="shrink-0 font-mono tabular-nums text-stream-text hover:bg-stream/12 hover:text-stream-text focus-visible:outline-stream"
          >
            <Heart className={cn('size-4', liked && 'fill-current')} aria-hidden />
            {likeCount.toLocaleString()}
          </Button>
        </div>
      </figcaption>

      <Dialog open={playerOpen} onOpenChange={setPlayerOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{item.title}</DialogTitle>
            <DialogDescription>
              {item.author} · {item.map}
            </DialogDescription>
          </DialogHeader>
          <video
            src={item.videoUrl}
            controls
            autoPlay
            playsInline
            preload="metadata"
            className="mx-auto max-h-[72vh] w-auto rounded-lg bg-surface-0"
          />
        </DialogContent>
      </Dialog>
    </figure>
  );
}
