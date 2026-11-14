"use client";

import { useEffect, useRef, useState } from "react";

const HUD_DESIGNS = [
  {
    id: "arena",
    name: "Arena",
    description: "Cian y ámbar, marcador compacto y placas rectas.",
  },
  {
    id: "apex",
    name: "Apex",
    description: "Azul hielo y rojo con cortes diagonales.",
  },
  {
    id: "prism",
    name: "Prism",
    description: "Violeta y menta sobre cristal oscuro.",
  },
  {
    id: "carbon",
    name: "Carbon",
    description: "Grafito y lima, filetes mínimos.",
  },
  {
    id: "pulse",
    name: "Pulse",
    description: "Celeste y rosa con esquinas suaves.",
  },
  {
    id: "royale",
    name: "Royale",
    description: "Azul profundo y oro, composición de torneo.",
  },
  {
    id: "glacier",
    name: "Glacier",
    description: "Superficies claras con detalles azul y coral.",
  },
  {
    id: "ember",
    name: "Ember",
    description: "Petróleo y cobre con placas escalonadas.",
  },
  {
    id: "circuit",
    name: "Circuit",
    description: "Turquesa y fucsia con esquinas técnicas.",
  },
  {
    id: "focus",
    name: "Focus",
    description: "Tarjeta inferior centrada con borde dorado y teclas WASD.",
  },
  {
    id: "mono",
    name: "Mono",
    description: "Monocromo y cifras de alto contraste.",
  },
] as const;

const HUD_COUNT = HUD_DESIGNS.length;
const AUTOPLAY_MS = 2600;
// The poster already carries a HUD at its edges, so the backdrop shows only its middle.
const GAMEPLAY = {
  backgroundImage: "url(/media/video/demo-completa.webp)",
  backgroundSize: "280% auto",
  backgroundPosition: "50% 55%",
} as const;

export default function HudCarousel() {
  const rootRef = useRef<HTMLElement>(null);
  const progressBarRef = useRef<HTMLSpanElement>(null);
  const progressRef = useRef(0);
  const [active, setActive] = useState(0);
  const [cycle, setCycle] = useState(0);
  const [inView, setInView] = useState(true);
  const [hovered, setHovered] = useState(false);
  const [focusWithin, setFocusWithin] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(false);

  const running = inView && !hovered && !focusWithin && !reducedMotion;
  const current = HUD_DESIGNS[active];
  const nextIndex = (active + 1) % HUD_COUNT;

  useEffect(() => {
    const root = rootRef.current;
    if (!root) return;
    const observer = new IntersectionObserver(
      ([entry]) => setInView(entry?.isIntersecting ?? true),
      { threshold: 0.2 },
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
    if (!reducedMotion) return;
    progressRef.current = 0;
    if (progressBarRef.current) progressBarRef.current.style.transform = "scaleX(0)";
  }, [reducedMotion]);

  // Progress is written straight to the bar so the carousel does not re-render every frame.
  useEffect(() => {
    if (!running) return;
    let frame = 0;
    let last = performance.now();

    const tick = (now: number) => {
      const elapsed = now - last;
      last = now;
      progressRef.current = Math.min(1, progressRef.current + elapsed / AUTOPLAY_MS);
      if (progressBarRef.current) {
        progressBarRef.current.style.transform = `scaleX(${progressRef.current})`;
      }
      if (progressRef.current >= 1) {
        progressRef.current = 0;
        if (progressBarRef.current) progressBarRef.current.style.transform = "scaleX(0)";
        setActive((value) => (value + 1) % HUD_COUNT);
        return;
      }
      frame = window.requestAnimationFrame(tick);
    };

    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [running, active, cycle]);

  function selectHud(index: number) {
    progressRef.current = 0;
    if (progressBarRef.current) progressBarRef.current.style.transform = "scaleX(0)";
    setActive(index);
    setCycle((value) => value + 1);
  }

  return (
    <section
      ref={rootRef}
      aria-label="Diseños de HUD"
      className="min-w-0"
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
      <div className="relative aspect-video w-full overflow-hidden border border-line bg-surface">
        <span aria-hidden="true" className="absolute inset-0 scale-105 opacity-80 blur-[2px]" style={GAMEPLAY} />
        <span className="absolute inset-0 bg-bg/30" aria-hidden="true" />

        {HUD_DESIGNS.map((hud, index) => {
          const isActive = index === active;
          return (
            <div
              key={hud.id}
              aria-hidden={!isActive}
              className={`absolute inset-0 z-10 transition-[opacity,transform] duration-700 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none ${
                isActive ? "opacity-100" : "opacity-0"
              }`}
              style={{ transform: isActive ? "scale(1)" : "scale(1.02)" }}
            >
              <img
                src={`/media/hud/${hud.id}.webp`}
                alt={`HUD ${hud.name} sobre una partida de Counter-Strike 2`}
                width={1920}
                height={1080}
                loading={isActive || index === nextIndex ? "eager" : "lazy"}
                decoding="async"
                className="size-full object-contain"
              />
            </div>
          );
        })}
      </div>

      <div className="h-0.5 overflow-hidden bg-line" aria-hidden="true">
        <span
          ref={progressBarRef}
          className="block h-full origin-left bg-accent"
          style={{ transform: "scaleX(0)" }}
        />
      </div>

      <div className="flex items-end justify-between gap-4 border-x border-b border-line bg-surface px-4 py-4 sm:px-5">
        <div className="min-w-0">
          <div className="flex items-baseline gap-2.5">
            <span className="size-1.5 shrink-0 bg-accent" aria-hidden="true" />
            <p className="font-display text-lg font-semibold leading-tight text-fg sm:text-xl">{current.name}</p>
          </div>
          <p className="mt-1.5 min-h-10 text-sm leading-snug text-muted sm:min-h-0">{current.description}</p>
        </div>
        <p className="shrink-0 font-mono text-xs tabular-nums text-muted">
          {String(active + 1).padStart(2, "0")} / {String(HUD_COUNT).padStart(2, "0")}
        </p>
      </div>

      <div
        role="group"
        aria-label="Elegir un diseño de HUD"
        className="mt-4 flex snap-x snap-proximity gap-3 overflow-x-auto scroll-px-1 px-1 pb-3 pt-1 [scrollbar-width:thin]"
      >
        {HUD_DESIGNS.map((hud, index) => {
          const isActive = index === active;
          return (
            <button
              key={hud.id}
              type="button"
              aria-pressed={isActive}
              aria-label={`Ver el HUD ${hud.name}`}
              onClick={() => selectHud(index)}
              className="group w-32 shrink-0 snap-start rounded-md text-left transition-transform duration-150 ease-out focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent motion-safe:active:scale-[0.97]"
            >
              <span
                className={`relative block aspect-video overflow-hidden rounded-md border bg-surface-2 transition-colors duration-150 ${
                  isActive
                    ? "border-accent ring-2 ring-accent ring-offset-2 ring-offset-bg"
                    : "border-line [@media(hover:hover)]:group-hover:border-muted/60"
                }`}
                style={GAMEPLAY}
              >
                <img
                  src={`/media/hud/${hud.id}.webp`}
                  alt=""
                  width={1920}
                  height={1080}
                  loading="lazy"
                  decoding="async"
                  className="size-full object-contain"
                />
              </span>
              <span className={`mt-1.5 block text-xs ${isActive ? "font-medium text-fg" : "text-muted"}`}>
                {hud.name}
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
