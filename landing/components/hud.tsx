import FilmBackdrop from "@/components/film";
import type { Film } from "@/lib/content";

interface PhoneProps {
  film: Film;
  badge: string;
  position: string;
  startAt: number;
  featured: boolean;
}

const FAN = ["-rotate-[7deg] translate-y-6", "z-10 scale-110", "rotate-[7deg] translate-y-6"] as const;

export function PhoneClip({ film, badge, position, startAt, featured }: PhoneProps) {
  return (
    <div className="relative aspect-[9/16] w-full overflow-hidden rounded-[1.4rem] border-2 border-fg/25 bg-surface shadow-[0_0_40px_rgb(34_224_124/0.18)]">
      <FilmBackdrop film={film} priority={featured} position={position} startAt={startAt} />
      <div aria-hidden="true" className="hud-scanlines absolute inset-0" />
      <div aria-hidden="true" className="absolute inset-x-0 top-0 flex items-center justify-between p-2.5 font-mono text-[9px] uppercase tracking-wider text-fg sm:p-3 sm:text-[10px]">
        <span className="flex items-center gap-1.5">
          <span className="hud-blink size-2 rounded-full bg-rec" />
          Rec
        </span>
        <span>9:16</span>
      </div>
      <div aria-hidden="true" className="absolute inset-x-2 top-[38%] flex justify-center">
        <span className="hud-cut -skew-x-12 bg-brand whitespace-nowrap px-1.5 py-1 font-display text-[10px] font-black sm:px-2 uppercase italic tracking-[0.08em] text-bg sm:text-sm">
          {badge}
        </span>
      </div>
    </div>
  );
}

export function PhoneFan({ film, phones }: { film: Film; phones: readonly { badge: string; position: string; startAt: number }[] }) {
  return (
    <div className="mx-auto grid w-full max-w-[34rem] grid-cols-3 items-end gap-3 px-3 sm:gap-5">
      {phones.map((phone, i) => (
        <div key={phone.badge} aria-hidden={i === 1 ? undefined : true} className={FAN[i] ?? ""}>
          <PhoneClip film={film} badge={phone.badge} position={phone.position} startAt={phone.startAt} featured={i === 1} />
        </div>
      ))}
    </div>
  );
}

const CLIPS = [
  { track: 0, left: 4, width: 22, tone: "bg-side-ct" },
  { track: 0, left: 28, width: 30, tone: "bg-side-t" },
  { track: 0, left: 60, width: 18, tone: "bg-side-ct" },
  { track: 0, left: 80, width: 16, tone: "bg-brand" },
  { track: 1, left: 10, width: 40, tone: "bg-brand/70" },
  { track: 1, left: 54, width: 34, tone: "bg-side-t/70" },
] as const;

const WAVE = Array.from({ length: 64 }, (_, i) => 20 + Math.round(Math.abs(Math.sin(i * 1.7) * Math.cos(i * 0.4)) * 80));

export function Timeline() {
  return (
    <div aria-hidden="true" className="hud-corners relative mx-auto mt-14 w-full max-w-[36rem] border border-line bg-surface/80 p-3 backdrop-blur">
      <div className="flex justify-between font-mono text-[10px] uppercase tracking-wider text-faint">
        <span>00:00:00</span>
        <span className="text-brand">Render 9:16</span>
        <span>00:00:42</span>
      </div>
      <div className="relative mt-2 space-y-1.5">
        {[0, 1].map((track) => (
          <div key={track} className="relative h-5 bg-line/60">
            {CLIPS.filter((c) => c.track === track).map((c) => (
              <span
                key={c.left}
                className={`absolute inset-y-0.5 ${c.tone}`}
                style={{ left: `${c.left}%`, width: `${c.width}%` }}
              />
            ))}
          </div>
        ))}
        <div className="flex h-7 items-center gap-px bg-line/40 px-1">
          {WAVE.map((h, i) => (
            <span key={i} className="w-full bg-brand/60" style={{ height: `${h}%` }} />
          ))}
        </div>
        <span className="hud-playhead absolute -inset-y-1 w-0.5 bg-rec shadow-[0_0_10px_var(--color-rec)]" />
      </div>
    </div>
  );
}

export function Ticker({ items }: { items: readonly string[] }) {
  const loop = [...items, ...items];
  return (
    <div aria-hidden="true" className="overflow-hidden border-y border-line bg-surface py-3">
      <ul className="hud-ticker flex w-max gap-10 whitespace-nowrap font-display text-sm font-extrabold uppercase italic tracking-[0.2em]">
        {loop.map((item, i) => (
          <li key={`${item}-${i}`} className="flex items-center gap-10">
            <span className={i % 2 === 0 ? "text-fg" : "text-brand"}>{item}</span>
            <span className="size-1.5 rotate-45 bg-side-t" />
          </li>
        ))}
      </ul>
    </div>
  );
}
