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
        compact ? "rounded-lg" : "rounded-xl"
      }`}
      style={{ backgroundImage: "linear-gradient(145deg, #25312a 0%, #111713 58%, #080b09 100%)" }}
    >
      <span
        aria-hidden="true"
        className="absolute inset-0 opacity-60"
        style={{
          backgroundImage:
            "linear-gradient(rgb(238 243 240 / 0.05) 1px, transparent 1px), linear-gradient(90deg, rgb(238 243 240 / 0.05) 1px, transparent 1px)",
          backgroundSize: "1.5rem 1.5rem",
        }}
      />
      <svg
        aria-hidden="true"
        viewBox="0 0 160 120"
        preserveAspectRatio="xMidYMax meet"
        className="absolute inset-x-0 bottom-0 h-[78%] w-full"
      >
        <circle cx="80" cy="38" r="22" fill="#84928a" />
        <rect x="31" y="62" width="98" height="66" rx="31" fill="#68776f" />
      </svg>
      <span
        className={`absolute left-2 top-2 inline-flex items-center gap-1 rounded-sm bg-accent font-mono font-semibold uppercase tracking-[0.12em] text-accent-fg ${
          compact ? "px-1 py-0.5 text-[7px]" : "px-1.5 py-1 text-[8px]"
        }`}
      >
        <span className="size-1 rounded-full bg-current" aria-hidden="true" />
        EN DIRECTO
      </span>
      <span
        className={`absolute bottom-2 left-2 font-mono uppercase tracking-[0.14em] text-fg/80 ${
          compact ? "text-[8px]" : "text-[9px]"
        }`}
      >
        Tu cámara
      </span>
      <span
        aria-hidden="true"
        className="absolute inset-x-2 bottom-1 h-px bg-white/15"
      />
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
      className="mx-auto w-full max-w-[17rem]"
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
        className="grid grid-cols-3 overflow-hidden rounded-md border border-line bg-surface"
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
              aria-controls={panelId}
              tabIndex={isActive ? 0 : -1}
              onClick={() => selectLayout(index)}
              onKeyDown={(event) => onTabKeyDown(event, index)}
              className={`flex min-h-11 items-center justify-center border-b-2 px-1.5 text-center text-[11px] font-medium leading-tight focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent ${
                isActive
                  ? "border-accent bg-surface-2 text-fg"
                  : "border-transparent text-muted hover:bg-surface-2/60 hover:text-fg"
              }`}
            >
              {layout.name}
            </button>
          );
        })}
      </div>

      <div
        role="tabpanel"
        id={panelId}
        aria-labelledby={`${panelId}-tab-${current.id}`}
        tabIndex={0}
        className="mt-4 rounded-md focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
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
              className="absolute inset-x-2.5 top-2.5 h-[31%] transition-[transform,opacity] duration-700 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none"
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
              <span className="inline-flex items-center gap-1.5 rounded-full border border-line bg-surface-2 px-3 py-1.5 font-mono text-[10px] text-fg">
                <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
                @tucanal
              </span>
            </div>

            <span
              aria-hidden="true"
              className="pointer-events-none absolute inset-x-0 top-0 flex items-center justify-between p-1.5 font-mono text-[7px] uppercase tracking-[0.14em] text-fg/45"
            >
              <span>9:16</span>
              <span>SHORT</span>
            </span>
          </div>

          <figcaption className="mt-3 min-h-5 text-center font-mono text-[11px] leading-relaxed text-muted">
            {current.description}
          </figcaption>
        </figure>
      </div>
    </section>
  );
}
