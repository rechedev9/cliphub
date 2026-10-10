import { Badge } from "@/components/ui/badge";

import type { JobStatus, Stage } from "./api-types";
import { STAGE_WORDS, STATUS_LABELS } from "./labels";

const VARIANTS = {
  awaiting_demo: "outline",
  queued: "info",
  running: "stream",
  uploading: "warning",
  done: "success",
  failed: "danger",
  canceled: "secondary",
  pending: "outline",
  approved: "warning",
  processing: "warning",
  rejected: "danger",
} as const satisfies Record<JobStatus, string>;

export interface StatusPillProps {
  status: JobStatus;
  // For a running job the pill names what the worker is doing with it.
  stage?: Stage | null;
}

export function StatusPill({ status, stage = null }: StatusPillProps) {
  const label = status === "running" && stage !== null ? STAGE_WORDS[stage] : STATUS_LABELS[status];
  return (
    <Badge variant={VARIANTS[status]} className="font-mono uppercase">
      {label}
    </Badge>
  );
}
