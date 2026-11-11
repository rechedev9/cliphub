"use client";

import { useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";

const LAYOUTS = [
  {
    id: "camara-juego",
    name: "Cámara + juego",
    description: "Tu cámara arriba, la partida abajo.",
  },
  {
    id: "camara-doble",
    name: "Cámara doble",
    description: "Cámara arriba y otra sobre la partida.",
  },
  {
    id: "solo-juego",
    name: "Solo juego",
    description: "Solo juego, con tu banner al pie.",
  },
] as const;

const LAYOUT_COUNT = LAYOUTS.length;
const AUTOPLAY_MS = 3000;

const GAME_TRANSFORMS = ["scale(1.04) translateY(4%)", "scale(1.07) translateY(6%)", "scale(1) translateY(0)"];

interface CameraFeedProps {
  compact?: boolean;
}

function CameraFeed({ compact = false }: CameraFeedProps) {
  return (
    <div
      className={`relative size-full overflow-hidden border border-white/10 bg-surface ${
        compact ? "rounded-lg" : "border-x-0 border-t-0"
      }`}
      style={{ backgroundImage: "linear-gradient(145deg, #25312a 0%, #111713 58%, #080b09 100%)" }}
    >
      <svg
        aria-hidden="true"
        viewBox="0 0 160 120"
        preserveAspectRatio="xMidYMax meet"
        className="absolute inset-x-0 bottom-0 h-[78%] w-full"
      >
        <circle cx="80" cy="38" r="22" fill="#84928a" />
        <rect x="31" y="62" width="98" height="66" rx="31" fill="#68776f" />
      </svg>
      {compact ? null : (
        <span className="absolute bottom-2 left-3 inline-flex items-center gap-1.5 rounded-sm bg-black/55 px-1.5 py-0.5 text-xs text-fg">
          <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
          Tu cámara
        </span>
      )}
    </div>
  );
}

export default function StreamLayouts() {
  const rootRef = useRef<HTMLElement>(null);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const panelId = useId();
  const [active, setActive] = useState(0);
  const [cycle, setCycle] = useState(0);
  const [inView, setInView] = useState(true);
  const [hovered, setHovered] = useState(false);
  const [focusWithin, setFocusWithin] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(false);

  const running = inView && !hovered && !focusWithin && !reducedMotion;
  const current = LAYOUTS[active];
  const primaryHidden = active === 2;
  const secondaryVisible = active === 1;
  const bannerVisible = active === 2;

  useEffect(() => {
    const root = rootRef.current;
    if (!root) return;
    const observer = new IntersectionObserver(
      ([entry]) => setInView(entry?.isIntersecting ?? true),
      { threshold: 0.25 },
    );
    observer.observe(root);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const sync = () => setReducedMotion(media.matches);
    sync();
    media.addEventListener("change", sync);
    return () => media.removeEventListener("change", sync);
  }, []);

  useEffect(() => {
    if (!running) return;
    const timer = window.setTimeout(() => {
      setActive((value) => (value + 1) % LAYOUT_COUNT);
    }, AUTOPLAY_MS);
    return () => window.clearTimeout(timer);
  }, [running, active, cycle]);

  function selectLayout(index: number) {
    setActive(index);
    setCycle((value) => value + 1);
  }

  function onTabKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>, index: number) {
    let next = index;
    if (event.key === "ArrowRight") next = (index + 1) % LAYOUT_COUNT;
    else if (event.key === "ArrowDown") next = (index + 1) % LAYOUT_COUNT;
    else if (event.key === "ArrowUp") next = (index - 1 + LAYOUT_COUNT) % LAYOUT_COUNT;
    else if (event.key === "ArrowLeft") next = (index - 1 + LAYOUT_COUNT) % LAYOUT_COUNT;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = LAYOUT_COUNT - 1;
    else return;
    event.preventDefault();
    selectLayout(next);
    tabRefs.current[next]?.focus();
  }

  return (
    <section
      ref={rootRef}
      aria-label="Diseños de Short"
      className="mx-auto grid w-full max-w-[20rem] gap-4 lg:max-w-none lg:grid-cols-[minmax(0,1fr)_16rem] lg:items-center lg:gap-6"
      onPointerEnter={() => setHovered(true)}
      onPointerLeave={() => setHovered(false)}
      onFocus={(event) => {
        const target = event.target;
        if (target instanceof HTMLElement) setFocusWithin(target.matches(":focus-visible"));
      }}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget instanceof Node ? event.relatedTarget : null)) {
          setFocusWithin(false);
        }
      }}
    >
      <div
        role="tablist"
        aria-label="Elegir el diseño del Short"
        className="grid grid-cols-3 overflow-hidden rounded-md border border-line bg-surface lg:grid-cols-1 lg:divide-y lg:divide-line"
      >
        {LAYOUTS.map((layout, index) => {
          const isActive = index === active;
          return (
            <button
              key={layout.id}
              ref={(element) => {
                tabRefs.current[index] = element;
              }}
              type="button"
              role="tab"
              id={`${panelId}-tab-${layout.id}`}
              aria-selected={isActive}
              tabIndex={isActive ? 0 : -1}
              onClick={() => selectLayout(index)}
              onKeyDown={(event) => onTabKeyDown(event, index)}
              className={`relative flex min-h-11 flex-col justify-center px-1 text-center transition-colors duration-150 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent lg:px-4 lg:py-3 lg:text-left ${
                isActive
                  ? "bg-surface-2 text-fg after:absolute after:inset-x-0 after:bottom-0 after:border-b-2 after:border-accent lg:after:inset-x-auto lg:after:inset-y-0 lg:after:left-0 lg:after:border-b-0 lg:after:border-l-2"
                  : "text-muted [@media(hover:hover)]:hover:bg-surface-2/60 [@media(hover:hover)]:hover:text-fg"
              }`}
            >
              <span className="whitespace-nowrap text-[13px] font-medium leading-tight lg:text-sm">{layout.name}</span>
              <span className="mt-1 hidden text-sm leading-snug text-muted lg:block">{layout.description}</span>
            </button>
          );
        })}
      </div>

      <div
        role="tabpanel"
        id={panelId}
        aria-labelledby={`${panelId}-tab-${current.id}`}
        tabIndex={0}
        className="rounded-md focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
      >
        <figure className="mx-auto w-full max-w-[17rem]">
          <div className="relative aspect-[9/16] w-full overflow-hidden rounded-[1.35rem] border border-line bg-black">
            <img
              src="/media/video/short.webp"
              alt="Jugada de Counter-Strike 2 en Mirage grabada en vertical"
              width={432}
              height={768}
              loading="lazy"
              decoding="async"
              className="absolute inset-0 size-full object-cover transition-transform duration-700 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none"
              style={{ transform: GAME_TRANSFORMS[active] }}
            />
            <span
              aria-hidden="true"
              className="absolute inset-x-0 bottom-0 h-24 bg-linear-to-t from-black/75 to-transparent"
            />

            <div
              aria-hidden={primaryHidden}
              className="absolute inset-x-0 top-0 h-[34%] transition-[transform,opacity] duration-700 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none"
              style={{
                transform: primaryHidden ? "translateY(-10px) scale(0.94)" : "translateY(0) scale(1)",
                opacity: primaryHidden ? 0 : 1,
              }}
            >
              <CameraFeed />
            </div>

            <div
              aria-hidden={!secondaryVisible}
              className="absolute bottom-4 right-3 aspect-[4/5] w-[36%] transition-[transform,opacity] duration-700 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none"
              style={{
                transform: secondaryVisible ? "translateY(0) scale(1)" : "translateY(10px) scale(0.82)",
                opacity: secondaryVisible ? 1 : 0,
              }}
            >
              <CameraFeed compact />
            </div>

            <div
              aria-hidden={!bannerVisible}
              className="absolute inset-x-0 bottom-3 flex justify-center transition-[transform,opacity] duration-700 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none"
              style={{
                transform: bannerVisible ? "translateY(0) scale(1)" : "translateY(8px) scale(0.94)",
                opacity: bannerVisible ? 1 : 0,
              }}
            >
              <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-surface-2 px-3 py-1.5 font-mono text-xs text-fg">
                <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
                @tucanal
              </span>
            </div>

          </div>

          <figcaption className="mt-3 min-h-5 text-center text-sm text-muted lg:hidden">
            {current.description}
          </figcaption>
        </figure>
      </div>
    </section>
  );
}
