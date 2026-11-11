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
        <span className="flex shrink-0 items-center gap-1.5" aria-hidden="true">
          <span className="size-2 rounded-full bg-[#ff5f57]" />
          <span className="size-2 rounded-full bg-[#febc2e]" />
          <span className="size-2 rounded-full bg-[#28c840]" />
        </span>
        <span className="truncate font-mono text-[11px] text-muted">{title}</span>
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
