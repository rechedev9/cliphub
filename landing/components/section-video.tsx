"use client";

import { useEffect, useRef, useState } from "react";

interface SectionVideoProps {
  name: string;
  label: string;
  aspect: "16/9" | "9/16";
  eager?: boolean;
}

/** Plays only while visible, and never for reduced motion: there it leaves the native controls. */
export default function SectionVideo({ name, label, aspect, eager = false }: SectionVideoProps) {
  const ref = useRef<HTMLVideoElement>(null);
  const [reduced, setReduced] = useState(false);
  const poster = `/media/video/${name}.webp`;

  useEffect(() => {
    const video = ref.current;
    if (!video) return;
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    let observer: IntersectionObserver | undefined;

    const sync = () => {
      setReduced(motion.matches);
      if (motion.matches) {
        observer?.disconnect();
        observer = undefined;
        video.pause();
        return;
      }
      if (!observer) {
        observer = new IntersectionObserver(
          ([entry]) => {
            if (!entry) return;
            if (entry.isIntersecting) void video.play().catch(() => undefined);
            else video.pause();
          },
          { threshold: 0.25 },
        );
        observer.observe(video);
      }
    };

    sync();
    motion.addEventListener("change", sync);
    return () => {
      observer?.disconnect();
      motion.removeEventListener("change", sync);
    };
  }, []);

  return (
    <>
      {eager ? <link rel="preload" as="image" href={poster} fetchPriority="high" /> : null}
      <video
        ref={ref}
        className={`block w-full bg-black object-cover ${aspect === "16/9" ? "aspect-[16/9]" : "aspect-[9/16]"}`}
        width={aspect === "16/9" ? 1280 : 720}
        height={aspect === "16/9" ? 720 : 1280}
        poster={poster}
        muted
        loop
        playsInline
        controls={reduced}
        preload="none"
        aria-label={label}
      >
        <source src={`/media/video/${name}.webm`} type="video/webm" />
        <source src={`/media/video/${name}.mp4`} type="video/mp4" />
      </video>
    </>
  );
}
