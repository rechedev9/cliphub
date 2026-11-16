"use client";

import { useEffect, useRef } from "react";
import type { Film } from "@/lib/content";

interface FilmBackdropProps {
  film: Film;
  priority?: boolean;
}

/** A full-bleed muted loop that plays only while on screen; reduced motion keeps the still frame. */
export default function FilmBackdrop({ film, priority = false }: FilmBackdropProps) {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = ref.current;
    if (!video || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) void video.play().catch(() => undefined);
        else video.pause();
      },
      { threshold: 0.15 },
    );
    observer.observe(video);
    return () => observer.disconnect();
  }, []);

  return (
    <video
      ref={ref}
      className="absolute inset-0 size-full object-cover"
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
