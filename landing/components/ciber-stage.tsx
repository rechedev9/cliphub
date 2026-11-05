"use client";

import { useEffect, useRef, useState } from "react";
import { STATIONS, TOUR, isStationId, stationById } from "@/lib/ciber/stations";
import type { StationId } from "@/lib/ciber/stations";
import type { AnchorPoint, CiberScene, Insets } from "@/lib/ciber/scene";

const LABELLED = STATIONS.filter((s) => s.anchor);

function stationFromHash(): StationId {
  const id = window.location.hash.slice(1);
  return isStationId(id) ? id : "inicio";
}

function readVisit(): number {
  try {
    const visits = Number(window.localStorage.getItem("ciber-visitas") ?? "0") + 1;
    window.localStorage.setItem("ciber-visitas", String(visits));
    return visits;
  } catch {
    return 1;
  }
}

function setLite(lite: boolean) {
  try {
    window.localStorage.setItem("ciber-lite", lite ? "1" : "0");
  } catch {
    // Private windows can refuse storage; the reload still applies the choice through the query.
  }
  const url = new URL(window.location.href);
  url.searchParams.delete("lite");
  url.searchParams.delete("3d");
  url.searchParams.set(lite ? "lite" : "3d", "1");
  window.location.replace(url.toString());
}

// The home stills are this much wider than their framed shot, so they reach under the panel (puesto07.py OVERSCAN).
const STILL_OVERSCAN = { wide: 1.5, tall: 2.1 } as const;

/** Place the loading still so its framed shot covers the free region, exactly like the first 3D frame. */
function placeStill(insets: Insets) {
  const stage = document.querySelector(".ciber-stage");
  if (!(stage instanceof HTMLElement)) return;
  const w = window.innerWidth - insets.right;
  const h = Math.max(1, window.innerHeight - insets.top - insets.bottom);
  const shape = w / h < 1 ? "tall" : "wide";
  stage.dataset.shape = shape;
  const home = stationById("inicio");
  const aspect = (shape === "tall" ? home.tall.still : home.wide.still) ?? w / h;
  const frameWidth = Math.max(w, h * aspect);
  const width = frameWidth * STILL_OVERSCAN[shape];
  const height = frameWidth / aspect;
  const img = stage.querySelector(`.ciber-still img[data-shape="${shape}"]`);
  if (!(img instanceof HTMLElement)) return;
  img.style.width = `${width}px`;
  img.style.height = `${height}px`;
  img.style.left = `${(w - width) / 2}px`;
  img.style.top = `${insets.top + (h - height) / 2}px`;
  img.dataset.placed = "true";
}

/** The free region is what the panel and, over a bottom sheet, the header and tools leave of the screen. */
function panelInsets(id: StationId): Insets {
  const panel = document.querySelector(`[data-panel="${id}"]`);
  if (!(panel instanceof HTMLElement)) return { top: 0, right: 0, bottom: 0 };
  // Offsets ignore the panel's entrance transform.
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  if (panel.offsetWidth < vw * 0.7) return { top: 0, right: Math.max(0, vw - panel.offsetLeft), bottom: 0 };
  const tools = document.querySelector(".ciber-tools");
  const top = tools instanceof HTMLElement ? tools.getBoundingClientRect().bottom + 4 : 0;
  return { top, right: 0, bottom: Math.max(0, vh - panel.offsetTop) };
}

export default function CiberStage() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const labelRefs = useRef(new Map<StationId, HTMLAnchorElement>());
  const sceneRef = useRef<CiberScene | null>(null);
  const [progress, setProgress] = useState(0);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    const root = document.documentElement;
    const canvas = canvasRef.current;
    if (root.dataset.ciber !== "3d" || !canvas) return;
    let alive = true;
    let observed: Element | null = null;
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const coarse =
      window.matchMedia("(pointer: coarse)").matches || Math.min(window.innerWidth, window.innerHeight) < 700;
    root.dataset.station = "inicio";
    setPaused(reducedMotion);

    function applyInsets(id: StationId) {
      const insets = panelInsets(id);
      sceneRef.current?.setInsets(insets);
      if (id === "inicio") placeStill(insets);
    }
    const resizeObserver = new ResizeObserver(() => applyInsets(stationFromHashOrRoot()));
    function stationFromHashOrRoot(): StationId {
      const id = root.dataset.station ?? "inicio";
      return isStationId(id) ? id : "inicio";
    }
    function watchPanel(id: StationId) {
      if (observed) resizeObserver.unobserve(observed);
      observed = document.querySelector(`[data-panel="${id}"]`);
      if (observed) resizeObserver.observe(observed);
      applyInsets(id);
    }

    function show(id: StationId, instant: boolean) {
      root.dataset.station = id;
      sceneRef.current?.goTo(id, instant);
      watchPanel(id);
      const heading = document.getElementById(`${id}-titulo`);
      if (heading && id !== "inicio") heading.focus({ preventScroll: true });
    }

    watchPanel("inicio");
    void import("@/lib/ciber/scene").then(({ createCiberScene }) => {
      if (!alive) return;
      const scene = createCiberScene({
        canvas,
        quality: coarse ? "low" : "high",
        reducedMotion,
        posterVariant: ((readVisit() - 1) % 3) + 1,
        onProgress: (ratio) => setProgress(ratio),
        onHover: (id) => {
          for (const [key, el] of labelRefs.current) el.dataset.hover = key === id ? "true" : "false";
        },
        onSelect: (id) => {
          window.location.hash = id;
        },
        onAnchors: (points: ReadonlyMap<StationId, AnchorPoint>) => {
          for (const [id, point] of points) {
            const el = labelRefs.current.get(id);
            if (!el) continue;
            // Keep the whole pill on screen when its object sits at the edge of the frame.
            const half = el.offsetWidth / 2 + 8;
            const x = Math.min(Math.max(point.x, half), window.innerWidth - half);
            el.style.transform = `translate3d(${x.toFixed(1)}px, ${point.y.toFixed(1)}px, 0) translate(-50%, -100%)`;
            el.dataset.visible = point.visible ? "true" : "false";
          }
        },
        onArrive: () => undefined,
        onError: () => {
          setFailed(true);
          root.dataset.ciber = "lite";
          sceneRef.current?.dispose();
          sceneRef.current = null;
        },
      });
      sceneRef.current = scene;
      applyInsets("inicio");
      scene.ready.then(
        () => {
          if (!alive) return;
          setReady(true);
          const first = stationFromHash();
          if (first !== "inicio") window.setTimeout(() => show(first, false), 500);
        },
        () => undefined,
      );
    });

    function onHash() {
      show(stationFromHash(), false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey) return;
      if (e.target instanceof HTMLElement && e.target.closest("input, textarea, select")) return;
      const current = stationFromHashOrRoot();
      const index = TOUR.indexOf(current);
      if (e.key === "Escape" && current !== "inicio") window.location.hash = "inicio";
      if (e.key === "ArrowRight") window.location.hash = TOUR[(index + 1) % TOUR.length];
      if (e.key === "ArrowLeft") window.location.hash = TOUR[index <= 0 ? TOUR.length - 1 : index - 1];
    }
    window.addEventListener("hashchange", onHash);
    window.addEventListener("keydown", onKey);
    return () => {
      alive = false;
      window.removeEventListener("hashchange", onHash);
      window.removeEventListener("keydown", onKey);
      resizeObserver.disconnect();
      sceneRef.current?.dispose();
      sceneRef.current = null;
    };
  }, []);

  function togglePause() {
    const next = !paused;
    setPaused(next);
    sceneRef.current?.setVideosPaused(next);
  }

  const percent = Math.round(progress * 100);
  return (
    <>
      {failed ? (
        <p className="mx-auto max-w-6xl px-5 pt-6 text-sm text-muted sm:px-8" role="status">
          No se pudo cargar la escena 3D, así que ves la versión ligera.
        </p>
      ) : null}
      <div className="ciber-stage" data-ready={ready ? "true" : "false"}>
        <div className="ciber-still" aria-hidden="true">
          <img src="/scene/stills/mesa.webp" alt="" data-shape="wide" />
          <img src="/scene/stills/mesa-v.webp" alt="" data-shape="tall" />
        </div>
        <canvas
          ref={canvasRef}
          className="ciber-canvas"
          role="img"
          aria-label="Un puesto de ciber de los años 2000: un monitor CRT con un Short de ClipHub, una torre con un CD, una libreta con notas de la partida y el ticket de la hora"
        />
        <div className="ciber-labels">
          {LABELLED.map((s) => (
            <a
              key={s.id}
              href={`#${s.id}`}
              className="ciber-label"
              data-visible="false"
              data-hover="false"
              ref={(el) => {
                if (el) labelRefs.current.set(s.id, el);
                else labelRefs.current.delete(s.id);
              }}
            >
              {s.label}
            </a>
          ))}
        </div>
        {!ready && !failed ? (
          <p className="ciber-loading" role="status">
            Encendiendo el ciber… {percent} %
          </p>
        ) : null}
        <div className="ciber-tools">
          <button type="button" className="ciber-tool" onClick={togglePause} aria-pressed={paused}>
            {paused ? "Reproducir vídeos" : "Pausar vídeos"}
          </button>
          <button type="button" className="ciber-tool" onClick={() => setLite(true)}>
            Versión ligera
          </button>
        </div>
      </div>
    </>
  );
}

export function ThreeDLink() {
  return (
    <button type="button" className="lite-3d-link" onClick={() => setLite(false)}>
      Ver el ciber en 3D
    </button>
  );
}
