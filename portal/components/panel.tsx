import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// A Card at the density of a list row: the operator panel and the job lists are made of these.
export function Panel({
  className,
  children,
  ...props
}: Omit<React.ComponentProps<typeof Card>, "elevation">) {
  return (
    <Card className="py-4" {...props}>
      <CardContent className={cn("flex flex-col gap-3 px-4 sm:px-5", className)}>{children}</CardContent>
    </Card>
  );
}
