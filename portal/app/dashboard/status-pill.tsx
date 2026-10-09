import { Badge } from "@/components/ui/badge";
import type { RequestStatus } from "@/db/schema";

const LABELS: Record<RequestStatus, string> = {
  awaiting_demo: "Subiendo",
  pending: "Pendiente de revisión",
  approved: "Aprobada",
  processing: "En proceso",
  done: "Lista",
  failed: "Error",
  rejected: "Rechazada",
};

const VARIANTS = {
  awaiting_demo: "outline",
  pending: "outline",
  approved: "warning",
  processing: "warning",
  done: "success",
  failed: "danger",
  rejected: "danger",
} as const satisfies Record<RequestStatus, string>;

function isRequestStatus(status: string): status is RequestStatus {
  return Object.hasOwn(LABELS, status);
}

export function StatusPill({ status }: { status: string }) {
  const known = isRequestStatus(status);
  return (
    <Badge variant={known ? VARIANTS[status] : "outline"} className="font-mono uppercase">
      {known ? LABELS[status] : status}
    </Badge>
  );
}
