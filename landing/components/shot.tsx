interface ShotProps {
  name: string;
  alt: string;
  title: string;
}

/** A real Studio capture inside a restrained window frame. */
export default function Shot({ name, alt, title }: ShotProps) {
  return (
    <figure className="overflow-hidden rounded-xl border border-line bg-surface shadow-xl shadow-black/30">
      <div className="flex items-center gap-2 border-b border-line bg-surface-2 px-3 py-2">
        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted">{title}</span>
        <span className="flex shrink-0 items-center gap-3 text-muted/70" aria-hidden="true">
          <span className="h-px w-2.5 bg-current" />
          <span className="size-2.5 border border-current" />
          <span className="relative size-2.5 before:absolute before:left-1/2 before:top-0 before:h-full before:w-px before:-translate-x-1/2 before:rotate-45 before:bg-current after:absolute after:left-1/2 after:top-0 after:h-full after:w-px after:-translate-x-1/2 after:-rotate-45 after:bg-current" />
        </span>
      </div>
      <img
        src={`/media/shots/${name}.webp`}
        alt={alt}
        width={1440}
        height={900}
        loading="lazy"
        decoding="async"
        className="aspect-[16/10] w-full object-cover"
      />
    </figure>
  );
}
