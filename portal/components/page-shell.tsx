import { cn } from "@/lib/utils";

// One measure per kind of page: prose, a list of requests, the landing.
const WIDTHS = {
  read: "max-w-2xl",
  list: "max-w-[60rem]",
  wide: "max-w-[70rem]",
} as const;

export function PageShell({
  width = "list",
  className,
  children,
}: {
  width?: keyof typeof WIDTHS;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <main className={cn("mx-auto w-full px-6 pt-12 pb-16", WIDTHS[width], className)}>
      {children}
    </main>
  );
}

export function PageHeader({
  title,
  children,
}: {
  title: string;
  children?: React.ReactNode;
}) {
  return (
    <header className="mb-8 flex items-center justify-between gap-4 border-b border-border-subtle pb-5">
      <h1 className="text-body-lg font-semibold tracking-[0.01em]">{title}</h1>
      <div className="flex gap-2">{children}</div>
    </header>
  );
}
