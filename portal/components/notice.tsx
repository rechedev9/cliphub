import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const noticeVariants = cva(
  "flex flex-wrap items-center justify-between gap-x-4 gap-y-2.5 rounded-md border px-4 py-3 text-body-sm text-fg-1",
  {
    variants: {
      tone: {
        danger: "border-destructive/45 bg-destructive/10",
        warning: "border-warning/45 bg-warning/10",
        info: "border-border-accent bg-primary/8",
      },
    },
    defaultVariants: { tone: "danger" },
  },
);

// A message that interrupts the page: a failed load, a stopped worker, a warning before acting.
export function Notice({
  tone,
  className,
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof noticeVariants>) {
  return <div data-slot="notice" className={cn(noticeVariants({ tone }), className)} {...props} />;
}

export function NoticeDetail({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("mt-1 text-fg-2 wrap-anywhere", className)} {...props} />;
}
