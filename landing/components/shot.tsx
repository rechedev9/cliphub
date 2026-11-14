import type { Shot } from "@/lib/content";

interface ShotFigureProps {
  shot: Shot;
  priority?: boolean;
}

/** A real Studio screenshot with the numbers worth reading pulled out beside it. */
export default function ShotFigure({ shot, priority = false }: ShotFigureProps) {
  return (
    <figure className="overflow-hidden rounded-lg border border-line bg-surface">
      <a href={shot.src} className="block border-b border-line">
        <img
          src={shot.src}
          alt={shot.alt}
          width={1600}
          height={1200}
          loading={priority ? "eager" : "lazy"}
          fetchPriority={priority ? "high" : "auto"}
          decoding="async"
          className="block h-auto w-full"
        />
      </a>
      <figcaption className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-sm text-muted">{shot.caption}</span>
        <span className="flex flex-wrap gap-x-5 gap-y-1 font-mono text-xs">
          {shot.callouts.map((c) => (
            <span key={c.label}>
              <span className="text-brand">{c.value}</span> <span className="text-faint">{c.label}</span>
            </span>
          ))}
        </span>
      </figcaption>
    </figure>
  );
}
