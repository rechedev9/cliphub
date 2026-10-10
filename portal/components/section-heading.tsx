import { Badge } from "@/components/ui/badge";

export function SectionHeading({
  title,
  count,
  children,
}: {
  title: string;
  count?: number;
  // An optional link or note pushed to the far edge.
  children?: React.ReactNode;
}) {
  return (
    <div className="mt-10 mb-3 flex flex-wrap items-center gap-x-2.5 gap-y-1 first:mt-0">
      <h2 className="text-body-lg font-semibold">{title}</h2>
      {count !== undefined && (
        <Badge variant="secondary" className="font-mono tabular-nums">
          {count}
        </Badge>
      )}
      {children !== undefined && <div className="ml-auto text-body-sm">{children}</div>}
    </div>
  );
}
