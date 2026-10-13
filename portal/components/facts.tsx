import { cn } from "@/lib/utils";

// Label and value pairs, two abreast on a phone and as many as fit above.
export function Facts({
  wide = false,
  className,
  children,
}: {
  wide?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <dl
      className={cn(
        "grid grid-cols-2 gap-x-4 gap-y-3",
        wide
          ? "sm:grid-cols-[repeat(auto-fill,minmax(15rem,1fr))]"
          : "sm:grid-cols-[repeat(auto-fill,minmax(8.5rem,1fr))]",
        className,
      )}
    >
      {children}
    </dl>
  );
}

export function Fact({
  label,
  className,
  children,
}: {
  label: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="min-w-0">
      <dt className="font-mono text-meta tracking-[0.08em] text-fg-3 uppercase">{label}</dt>
      <dd className={cn("mt-0.5 wrap-anywhere", className)}>{children}</dd>
    </div>
  );
}
