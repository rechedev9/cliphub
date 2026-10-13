import { StatusDot, TONE_TEXT } from "@/components/status-dot";
import { cn } from "@/lib/utils";

import type { SummaryCell } from "../../components/queue-summary";
import { CELL_LABEL, MUTED, RULED } from "./styles";

export function Summary({ cells }: { cells: SummaryCell[] }) {
  return (
    <section className={cn(RULED, "mb-5 grid-cols-2 lg:grid-cols-4")} aria-label="Estado de la nube">
      {cells.map((cell) => {
        const body = (
          <>
            <span className={cn(CELL_LABEL, "flex items-center gap-2 wrap-anywhere")}>
              <StatusDot tone={cell.tone === "plain" ? "off" : cell.tone} className="size-2" />
              {cell.label}
            </span>
            <span
              className={cn(
                "text-body-lg leading-tight font-semibold wrap-anywhere sm:text-title",
                cell.tone !== "plain" && TONE_TEXT[cell.tone],
              )}
            >
              {cell.value}
            </span>
            {cell.detail !== null && <span className={cn(MUTED, "wrap-anywhere")}>{cell.detail}</span>}
          </>
        );
        const className = cn(
          "flex min-w-0 flex-col gap-1.5 bg-surface-2 px-3.5 py-3 text-fg-1 sm:px-4 sm:py-3.5",
          // The one cell that is in trouble is tinted, so it is found before it is read.
          cell.tone === "bad" && "bg-[color-mix(in_oklch,var(--destructive)_9%,var(--surface-2))]",
        );
        return cell.href === null ? (
          <div key={cell.key} className={className}>
            {body}
          </div>
        ) : (
          <a
            key={cell.key}
            className={cn(className, "transition-colors duration-(--dur-fast) hover:bg-surface-3")}
            href={cell.href}
          >
            {body}
          </a>
        );
      })}
    </section>
  );
}
