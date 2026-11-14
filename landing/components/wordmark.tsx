interface WordmarkProps {
  analytics?: boolean;
}

/** The ClipHub mark and name; the web edition adds "analytics" in small mono. */
export default function Wordmark({ analytics = false }: WordmarkProps) {
  return (
    <span className="inline-flex items-center gap-2">
      <img src="/brand/cliphub-mark.svg" alt="" width={28} height={28} className="size-7" />
      <span className="font-display text-xl font-extrabold uppercase tracking-[0.14em] text-fg">ClipHub</span>
      {analytics ? (
        <span className="translate-y-px font-mono text-[11px] uppercase tracking-[0.14em] text-brand">analytics</span>
      ) : null}
    </span>
  );
}
