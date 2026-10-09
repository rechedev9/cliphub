"use client";

import { useEffect, useRef, useState } from "react";
import { PauseIcon, PlayIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const ICON = "absolute size-3.5 fill-current transition-[opacity,scale] duration-(--dur-fast) ease-standard";
const ICON_OFF = "scale-80 opacity-0";

export function ReelPlayer() {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [playing, setPlaying] = useState(false);
  // Set once the visitor pauses by hand, so scrolling stops overriding them.
  const pausedByUser = useRef(false);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    // With reduced motion the poster stays put until the visitor presses play.
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries.at(-1);
        if (!entry) return;
        if (entry.isIntersecting && !pausedByUser.current) {
          void video.play().catch(() => undefined);
        } else {
          video.pause();
        }
      },
      { threshold: 0.3 },
    );
    observer.observe(video);
    return () => observer.disconnect();
  }, []);

  function toggle() {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      pausedByUser.current = false;
      void video.play().catch(() => undefined);
    } else {
      pausedByUser.current = true;
      video.pause();
    }
  }

  return (
    <>
      <video
        ref={videoRef}
        className="absolute inset-0 size-full object-cover"
        muted
        loop
        playsInline
        preload="metadata"
        poster="/media/reel-sample-poster.webp"
        aria-label="Muestra de un vídeo de ClipHub: una ronda de CS2 grabada en primera persona con el killfeed del juego"
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
      >
        <source src="/media/reel-sample.mp4" type="video/mp4" />
      </video>
      <Button
        type="button"
        variant="outline"
        size="icon-sm"
        className="absolute top-2 right-2 z-10 rounded-full border-border bg-surface-0/80 backdrop-blur-sm hover:bg-surface-0"
        aria-label={playing ? "Pausar la muestra" : "Reproducir la muestra"}
        onClick={toggle}
      >
        <PauseIcon aria-hidden className={cn(ICON, !playing && ICON_OFF)} />
        <PlayIcon aria-hidden className={cn(ICON, playing && ICON_OFF)} />
      </Button>
    </>
  );
}
