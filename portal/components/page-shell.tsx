import { cn } from "@/lib/utils";

// One measure per kind of page: prose, a list of jobs, the landing, the operator panel.
const WIDTHS = {
  read: "max-w-2xl",
  list: "max-w-[60rem]",
  wide: "max-w-[70rem]",
  panel: "max-w-[82rem]",
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
  className,
  children,
}: {
  title: string;
  className?: string;
  children?: React.ReactNode;
}) {
  return (
    <header
      className={cn(
        "mb-8 flex flex-wrap items-center justify-between gap-x-4 gap-y-3 border-b border-border-subtle pb-5",
        className,
      )}
    >
      <h1 className="text-body-lg font-semibold tracking-[0.01em]">{title}</h1>
      <div className="flex flex-wrap gap-2">{children}</div>
    </header>
  );
}
