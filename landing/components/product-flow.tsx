"use client";

import { useEffect, useState } from "react";

const LAST = 8;
const HOLD = 4;
const DECISIONS = ["Jugador", "Música", "Efectos"] as const;
const STATS = [
  { label: "ADR", value: "94", width: "78%" },
  { label: "KAST", value: "78 %", width: "64%" },
  { label: "Rating", value: "1,21", width: "70%" },
] as const;

/** The product in one loop: a demo goes in, Studio reads it, the person decides, a Short and the stats come out. */
export default function ProductFlow() {
  // Server render and reduced motion show the finished flow; the loop starts after hydration.
  const [step, setStep] = useState(LAST);

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    let tick = 0;
    setStep(0);
    const id = window.setInterval(() => {
      tick = tick >= LAST + HOLD ? 0 : tick + 1;
      setStep(Math.min(tick, LAST));
    }, 800);
    return () => window.clearInterval(id);
  }, []);

  const on = (k: number) => (step >= k ? "true" : "false");

  return (
    <figure
      className="flow"
      aria-label="Cómo funciona: cargas una demo, Studio la lee, tú eliges jugador, música y efectos, y sale un Short listo para subir junto con el análisis de la partida."
    >
      <ol aria-hidden="true" className="space-y-3">
        <li className="flow-row" data-on={on(0)}>
          <span className="flow-num">1</span>
          <div className="min-w-0 flex-1">
            <p className="flow-file">
              <span className="flow-dem">.dem</span>
              final_inferno.dem
            </p>
            <div className="flow-bar mt-2" data-on={on(1)}>
              <span />
            </div>
            <p className="flow-note mt-1.5" data-on={on(2)}>
              24 rondas · 186 kills · 10 jugadores
            </p>
          </div>
        </li>
        <li className="flow-row" data-on={on(3)}>
          <span className="flow-num">2</span>
          <div className="min-w-0 flex-1">
            <p className="flow-title">Tú eliges</p>
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {DECISIONS.map((d, i) => (
                <span key={d} className="flow-chip" data-on={on(3 + i)}>
                  {d}
                </span>
              ))}
            </div>
          </div>
        </li>
        <li className="flow-row" data-on={on(6)}>
          <span className="flow-num">3</span>
          <div className="grid min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)] gap-3">
            <div className="flow-short" data-on={on(6)}>
              <img src="/media/video/short.webp" alt="" width={54} height={96} loading="lazy" decoding="async" />
              <span>Short</span>
            </div>
            <div className="flow-stats" data-on={on(7)}>
              <p className="flow-title">Análisis</p>
              {STATS.map((s) => (
                <p key={s.label} className="flow-stat">
                  <span className="w-12 shrink-0">{s.label}</span>
                  <span className="flow-meter">
                    <span style={{ width: s.width }} />
                  </span>
                  <span className="w-9 shrink-0 text-right tabular-nums">{s.value}</span>
                </p>
              ))}
            </div>
          </div>
        </li>
      </ol>
      <p aria-hidden="true" className="flow-stamp" data-on={on(8)}>
        Listo para subir
      </p>
    </figure>
  );
}
