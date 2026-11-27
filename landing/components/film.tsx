"use client";

import { useEffect, useRef } from "react";
import type { Film } from "@/lib/content";

interface FilmBackdropProps {
  film: Film;
  priority?: boolean;
  position?: string;
  startAt?: number;
}

export default function FilmBackdrop({ film, priority = false, position = "50% 50%", startAt = 0 }: FilmBackdropProps) {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = ref.current;
    if (!video || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const seek = () => {
      if (startAt > 0 && video.currentTime < startAt) video.currentTime = startAt;
    };
    video.addEventListener("loadedmetadata", seek, { once: true });
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) void video.play().catch(() => undefined);
        else video.pause();
      },
      { threshold: 0.15 },
    );
    observer.observe(video);
    return () => {
      observer.disconnect();
      video.removeEventListener("loadedmetadata", seek);
    };
  }, [startAt]);

  return (
    <video
      ref={ref}
      className="absolute inset-0 size-full object-cover"
      style={{ objectPosition: position }}
      poster={`${film.src}.webp`}
      muted
      loop
      playsInline
      preload={priority ? "auto" : "none"}
      aria-label={film.label}
    >
      <source src={`${film.src}.webm`} type="video/webm" />
      <source src={`${film.src}.mp4`} type="video/mp4" />
    </video>
  );
}
