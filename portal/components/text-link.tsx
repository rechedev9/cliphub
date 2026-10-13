import { cn } from "@/lib/utils";

export function TextLink({ className, ...props }: React.ComponentProps<"a">) {
  return (
    <a
      className={cn(
        "text-primary underline decoration-primary/35 underline-offset-[0.2em] hover:decoration-current",
        className,
      )}
      {...props}
    />
  );
}
