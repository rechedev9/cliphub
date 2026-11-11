"use client";

import { useEffect, useRef, useState } from "react";
import {
  FileArchive,
  FileBox,
  Film,
  Gamepad2,
  KeyRound,
  Layers,
  Play,
  Radio,
  Swords,
  Trophy,
  Video,
} from "lucide-react";

const SOURCES = [
  { label: "Archivos .dem", icon: FileBox },
  { label: ".zip y .rar", icon: FileArchive },
  { label: "Código de Steam", icon: KeyRound },
  { label: "Tus partidas de Steam", icon: Gamepad2 },
  { label: "FACEIT", icon: Swords },
  { label: "Partidas pro", icon: Trophy },
  { label: "Series BO3 y BO5", icon: Layers },
  { label: "Twitch", icon: Radio },
  { label: "YouTube", icon: Play },
  { label: "Kick", icon: Video },
  { label: "Tu MP4", icon: Film },
] as const;

const MARQUEE_CSS = `
.import-marquee-cliphub {
  display: flex;
  width: max-content;
  animation: import-marquee-cliphub-scroll 40s linear infinite;
  animation-play-state: paused;
}

.import-marquee-cliphub[data-running="true"] {
  animation-play-state: running;
}

.import-marquee-cliphub:hover,
.import-marquee-cliphub:focus-within {
  animation-play-state: paused;
}

.import-marquee-cliphub-mask {
  mask-image: linear-gradient(to right, transparent, black 8%, black 92%, transparent);
}

@keyframes import-marquee-cliphub-scroll {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-50%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .import-marquee-cliphub-mask {
    mask-image: none;
  }

  .import-marquee-cliphub {
    animation: none;
    width: auto;
    flex-wrap: wrap;
    justify-content: center;
    row-gap: 0.5rem;
  }

  .import-marquee-cliphub [data-duplicate="true"] {
    display: none;
  }
}
`;

export default function ImportSources() {
  const trackRef = useRef<HTMLUListElement>(null);
  const [inView, setInView] = useState(false);
  const [reduceMotion, setReduceMotion] = useState(false);

  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const sync = () => setReduceMotion(query.matches);
    sync();
    query.addEventListener("change", sync);
    return () => query.removeEventListener("change", sync);
  }, []);

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    const observer = new IntersectionObserver(([entry]) => {
      setInView(entry?.isIntersecting === true);
    });
    observer.observe(track);
    return () => observer.disconnect();
  }, []);

  const copies = reduceMotion ? 1 : 2;

  return (
    <>
      <style>{MARQUEE_CSS}</style>
      <div className="import-marquee-cliphub-mask relative overflow-hidden py-1">
        <ul
          ref={trackRef}
          aria-label="Fuentes que puedes importar"
          data-running={inView ? "true" : "false"}
          className="import-marquee-cliphub"
        >
          {Array.from({ length: copies }, (_, copy) =>
            SOURCES.map(({ label, icon: Icon }) => {
              const duplicate = copy === 1;
              return (
                <li
                  key={`${copy}-${label}`}
                  aria-hidden={duplicate ? true : undefined}
                  data-duplicate={duplicate ? "true" : undefined}
                  className="shrink-0 pr-3"
                >
                  <span className="flex items-center gap-2 rounded-full border border-line bg-surface px-3.5 py-2 font-mono text-[11px] tracking-[0.08em] whitespace-nowrap text-muted uppercase">
                    <Icon
                      aria-hidden="true"
                      strokeWidth={1.75}
                      className="size-3.5 text-primary"
                    />
                    {label}
                  </span>
                </li>
              );
            }),
          )}
        </ul>
      </div>
    </>
  );
}
