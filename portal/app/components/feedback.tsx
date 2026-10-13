import { Loader2Icon } from "lucide-react";

import { Notice, NoticeDetail } from "@/components/notice";
import { Panel } from "@/components/panel";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { formatClock } from "./format";
import { loadErrorText } from "./labels";

export interface LoadErrorProps {
  error: { code: string; detail: string | null };
  onRetry: () => void;
  // When the data still on screen below the banner was loaded; null when there is none.
  staleSince: number | null;
}

export function LoadError({ error, onRetry, staleSince }: LoadErrorProps) {
  return (
    <Notice tone="danger" role="alert" className="mb-4">
      <div className="min-w-0 flex-1 basis-56">
        <strong className="font-semibold">{loadErrorText(error.code)}</strong>
        {staleSince !== null && (
          <span> Lo que ves abajo es de las {formatClock(staleSince)} y puede haber cambiado.</span>
        )}
        {error.detail !== null && <NoticeDetail className="font-mono">{error.detail}</NoticeDetail>}
      </div>
      {error.code === "unauthorized" ? (
        <Button asChild variant="outline" size="sm">
          <a href="/login">Entrar</a>
        </Button>
      ) : (
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          Reintentar
        </Button>
      )}
    </Notice>
  );
}

export function ActionError({ message }: { message: string | null }) {
  if (message === null) return null;
  return (
    <Notice tone="danger" role="alert" className="mb-4">
      {message}
    </Notice>
  );
}

export function Loading({ children }: { children: React.ReactNode }) {
  return (
    <p role="status" className="flex items-center gap-2 text-body-sm text-fg-2">
      <Loader2Icon aria-hidden className="size-4 animate-spin" />
      {children}
    </p>
  );
}

export function Empty({ children }: { children: React.ReactNode }) {
  return <Panel className="text-fg-2">{children}</Panel>;
}

const METER_FILL = {
  primary: "bg-primary",
  bad: "bg-destructive",
  live: "bg-stream",
  off: "bg-fg-4",
} as const;

export interface MeterProps {
  percent: number;
  // "off" is for values that are no longer being updated.
  tone?: keyof typeof METER_FILL;
  label: string;
}

export function Meter({ percent, tone = "primary", label }: MeterProps) {
  const clamped = Math.min(100, Math.max(0, percent));
  return (
    <div
      className="h-1.5 overflow-hidden rounded-full bg-surface-0 shadow-[var(--elev-0)]"
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(clamped)}
    >
      <div
        className={cn("h-full rounded-full transition-[width] duration-(--dur-slow) ease-standard", METER_FILL[tone])}
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}
