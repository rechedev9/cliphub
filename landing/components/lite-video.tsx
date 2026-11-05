"use client";

import { useEffect, useRef } from "react";

interface LiteVideoProps {
  src: string;
  poster: string;
  label: string;
}

/** The real Short, playing only while visible and never for reduced motion. */
export default function LiteVideo({ src, poster, label }: LiteVideoProps) {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = ref.current;
    if (!video || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry?.isIntersecting) void video.play().catch(() => undefined);
      else video.pause();
    });
    observer.observe(video);
    return () => observer.disconnect();
  }, []);

  return (
    <video
      ref={ref}
      className="aspect-[9/16] w-full max-w-[19rem] rounded-xl border border-line bg-black object-cover"
      poster={poster}
      muted
      loop
      playsInline
      controls
      preload="none"
      aria-label={label}
    >
      <source src={`${src}.webm`} type="video/webm" />
      <source src={`${src}.mp4`} type="video/mp4" />
    </video>
  );
}
