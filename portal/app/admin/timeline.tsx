import { formatDateTime } from "../components/format";
import { actorLabel } from "../components/labels";

export function Timeline({ children }: { children: React.ReactNode }) {
  return <ol className="text-body-sm">{children}</ol>;
}

export function TimelineItem({
  at,
  actor,
  children,
}: {
  at: number;
  actor: string;
  children: React.ReactNode;
}) {
  return (
    <li className="grid gap-x-4 gap-y-0.5 border-b border-border-subtle py-2.5 wrap-anywhere first:pt-0 last:border-b-0 last:pb-0 sm:grid-cols-[7.5rem_minmax(0,1fr)_auto]">
      <span className="font-mono text-fg-3 tabular-nums">{formatDateTime(at)}</span>
      <span>{children}</span>
      <span className="text-fg-3">{actorLabel(actor)}</span>
    </li>
  );
}
