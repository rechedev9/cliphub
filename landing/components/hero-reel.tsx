"use client";

import { useState } from "react";
import { Crosshair, Play, Skull, Target } from "lucide-react";

// The <video> is optional: while /video/hero-loop.webm is absent it errors out
// silently and the still carries the frame. Motion is CSS (see globals.css).

const KILLFEED = [
  { victim: "dev1ce", weapon: "AK-47", headshot: true, delay: "0s" },
  { victim: "s1mple", weapon: "AK-47", headshot: true, delay: "1.1s" },
  { victim: "ZywOo", weapon: "Deagle", headshot: false, delay: "2.3s" },
  { victim: "NiKo", weapon: "AK-47", headshot: true, delay: "3.4s" },
  { victim: "ropz", weapon: "AWP", headshot: false, delay: "4.6s" },
] as const;

function FrameCorners() {
  return (
    <>
      <span aria-hidden="true" className="pointer-events-none absolute -left-px -top-px h-5 w-5 border-l-2 border-t-2 border-orange-400" />
      <span aria-hidden="true" className="pointer-events-none absolute -right-px -top-px h-5 w-5 border-r-2 border-t-2 border-orange-400" />
      <span aria-hidden="true" className="pointer-events-none absolute -bottom-px -left-px h-5 w-5 border-b-2 border-l-2 border-orange-400" />
      <span aria-hidden="true" className="pointer-events-none absolute -bottom-px -right-px h-5 w-5 border-b-2 border-r-2 border-orange-400" />
    </>
  );
}

export default function HeroReel() {
  // Flipped when /video/hero-loop.webm 404s; the still then owns the frame.
  const [videoBroken, setVideoBroken] = useState(false);

  return (
    <div className="pointer-events-none relative hidden min-h-[600px] lg:block" aria-hidden="true">
      {/* Ambient glow pair behind the frame */}
      <div className="absolute right-[4%] top-1/2 size-[420px] -translate-y-1/2 rounded-full bg-orange-400/12 blur-[90px]" />
      <div className="absolute right-[26%] top-[18%] size-[240px] rounded-full bg-violet-500/10 blur-[80px]" />

      {/* Phone frame */}
      <div className="absolute right-[8%] top-1/2 w-[300px] -translate-y-1/2 xl:w-[340px]">
        <div className="relative aspect-[9/16] overflow-hidden border border-orange-400/40 bg-[#050812] shadow-[0_0_90px_rgba(251,146,60,0.22)]">
          {/* Gameplay layer: real short when present, still otherwise */}
          <div className="absolute inset-0 bg-[url('/images/hero-replay-forge.webp')] bg-cover bg-[position:76%_center]" />
          {!videoBroken && (
            <video
              className="absolute inset-0 size-full object-cover"
              src="/video/hero-loop.webm"
              autoPlay
              muted
              loop
              playsInline
              onError={() => setVideoBroken(true)}
            />
          )}
          <div className="absolute inset-0 bg-gradient-to-b from-[#050812]/55 via-transparent to-[#050812]/80" />

          {/* Scanline sweep */}
          <div className="absolute inset-x-0 top-0 h-16 bg-gradient-to-b from-transparent via-orange-400/12 to-transparent motion-safe:animate-[reel-scan_5s_linear_infinite]" />

          {/* Killfeed */}
          <div className="absolute right-2.5 top-2.5 flex flex-col items-end gap-1.5">
            {KILLFEED.map(({ victim, weapon, headshot, delay }) => (
              <div
                key={victim}
                style={{ animationDelay: delay }}
                className="flex items-center gap-1.5 border border-white/10 bg-slate-950/80 px-2 py-1 font-mono text-[9px] tracking-wide backdrop-blur-sm motion-safe:translate-x-[130%] motion-safe:animate-[reel-kill_9s_ease-out_infinite]"
              >
                <span className="font-semibold text-orange-400">rechewski</span>
                <span className="text-slate-500">[{weapon}]</span>
                {headshot && <Skull className="size-2.5 text-orange-400" strokeWidth={2.4} />}
                <span className="text-red-400 line-through decoration-red-400/60">{victim}</span>
              </div>
            ))}
          </div>

          {/* Ace chip */}
          <div className="absolute left-2.5 top-[38%] border border-orange-400/35 bg-slate-950/75 px-2.5 py-2 backdrop-blur-sm motion-safe:animate-[reel-chip_9s_ease-out_infinite]">
            <p className="font-mono text-[8px] uppercase tracking-[0.22em] text-slate-400">Live analysis</p>
            <div className="mt-1 flex items-center gap-2">
              <p className="text-xl font-bold leading-none text-white">5K</p>
              <Target className="size-4 text-orange-400" strokeWidth={1.8} />
            </div>
            <p className="mt-1 font-mono text-[8px] text-orange-400">ROUND 09 · INFERNO</p>
          </div>

          {/* Crosshair pulse at frame center */}
          <Crosshair className="absolute left-1/2 top-1/2 size-6 -translate-x-1/2 -translate-y-1/2 text-orange-400/70 motion-safe:animate-pulse" strokeWidth={1.4} />

          {/* Bottom render bar */}
          <div className="absolute inset-x-2.5 bottom-2.5 border border-white/12 bg-slate-950/85 p-2.5 backdrop-blur-sm">
            <div className="flex items-center gap-2">
              <span className="grid size-7 shrink-0 place-items-center bg-violet-500/15 text-violet-400">
                <Play className="size-3.5 fill-current" />
              </span>
              <div className="min-w-0">
                <p className="font-mono text-[8px] uppercase tracking-[0.2em] text-slate-400">Output forged</p>
                <p className="truncate text-xs font-semibold text-white">1080 × 1920 · 60 FPS</p>
              </div>
            </div>
            <div className="mt-2 h-1 overflow-hidden bg-white/10">
              <div className="h-full w-full origin-left bg-gradient-to-r from-orange-400 to-violet-500 motion-safe:animate-[reel-render_9s_linear_infinite]" />
            </div>
          </div>

          <FrameCorners />
        </div>

        {/* Frame caption */}
        <p className="mt-3 text-center font-mono text-[10px] uppercase tracking-[0.24em] text-slate-500">
          Vertical short · forged locally
        </p>
      </div>
    </div>
  );
}
