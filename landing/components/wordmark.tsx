export default function Wordmark() {
  return (
    <span className="inline-flex items-center gap-2">
      <img src="/brand/cliphub-mark.svg" alt="" width={28} height={28} className="size-7" />
      <span className="font-display text-xl font-extrabold uppercase italic tracking-[0.12em] text-fg">ClipHub</span>
      <span className="-skew-x-12 bg-brand px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-[0.16em] text-bg">
        Studio
      </span>
    </span>
  );
}
