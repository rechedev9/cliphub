"use client";

import { useState } from "react";

// The <video> mirrors hero-reel.tsx: real footage when it loads, still frame
// plus synthetic overlays if /video/killfeed-loop.webm errors out.

// Stand-in for the burned-in native killfeed; only drawn when the video is
// absent, matching the EliGE ace vs Fnatic footage this frame normally shows.
const KILLFEED = ["EliGEjj  AK-47  calme", "EliGEjj  AK-47  jambo0", "EliGEjj  AK-47  mezay", "EliGEjj  AK-47  mozes", "EliGEjj  AK-47  Naffly"];

function FrameCorners() {
  return (
    <>
      <span aria-hidden="true" className="pointer-events-none absolute -left-px -top-px h-4 w-4 border-l-2 border-t-2 border-orange-400" />
      <span aria-hidden="true" className="pointer-events-none absolute -bottom-px -right-px h-4 w-4 border-b-2 border-r-2 border-orange-400" />
    </>
  );
}

export default function KillfeedReel() {
  // Flipped when /video/killfeed-loop.webm 404s; the still then owns the frame.
  const [videoBroken, setVideoBroken] = useState(false);

  return (
    <div className="relative mx-auto aspect-[9/16] w-[min(78vw,330px)] border border-orange-400/35 bg-[url('/images/hero-replay-forge.webp')] bg-cover bg-[position:76%_center] p-4 shadow-[0_0_80px_rgba(251,146,60,0.18)]">
      {/* Gameplay layer: real short when present, still otherwise */}
      {!videoBroken && (
        <video
          className="absolute inset-0 size-full object-cover"
          src="/video/killfeed-loop.webm"
          autoPlay
          muted
          loop
          playsInline
          onError={() => setVideoBroken(true)}
        />
      )}
      <FrameCorners />
      {/* Bottom-only gradient keeps the export bar readable without dimming the footage */}
      <div className="absolute inset-0 bg-gradient-to-b from-transparent via-transparent to-slate-950/70" />
      <div className="relative flex items-center justify-between font-mono text-[10px] uppercase tracking-[0.18em] text-white/70">
        {videoBroken && <span>Round 09</span>}
        <span className="ml-auto inline-flex items-center gap-2 text-emerald-300"><span className="size-1.5 animate-pulse bg-emerald-300 motion-reduce:animate-none" />Live</span>
      </div>
      {/* Killfeed: only shown as a stand-in while the real footage is unavailable */}
      {videoBroken && (
        <div className="absolute right-4 top-[20%] grid gap-1.5 text-[10px] font-semibold sm:text-xs">
          {KILLFEED.map((kill) => (
            <div key={kill} className="border-l-2 border-orange-400 bg-slate-950/85 px-3 py-2 text-white shadow-lg backdrop-blur-sm">{kill}</div>
          ))}
        </div>
      )}
      <div className="absolute inset-x-4 bottom-4 border border-white/15 bg-slate-950/80 p-4 backdrop-blur-md">
        <div className="flex items-end justify-between">
          <div><p className="font-mono text-[10px] uppercase tracking-[0.22em] text-orange-400">Export ready</p><p className="mt-1 text-2xl font-bold text-white">ACE</p></div>
          <p className="font-mono text-xs text-slate-300">5 / 5</p>
        </div>
        <div className="mt-3 h-1 bg-white/10"><div className="h-full w-full bg-gradient-to-r from-orange-400 to-violet-500" /></div>
      </div>
    </div>
  );
}
