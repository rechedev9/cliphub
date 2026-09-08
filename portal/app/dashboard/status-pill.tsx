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

export function StatusPill({ status }: { status: string }) {
  const label = LABELS[status as RequestStatus] ?? status;
  const className =
    status === "done"
      ? "pill pill-done"
      : status === "failed" || status === "rejected"
        ? "pill pill-failed"
        : status === "approved" || status === "processing"
          ? "pill pill-active"
          : "pill";
  return <span className={className}>{label}</span>;
}
