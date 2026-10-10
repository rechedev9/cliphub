"use client";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import type { AdminJob } from "../components/api-types";
import { ConfirmButton } from "../components/confirm-button";
import type { ActionRunner } from "../components/use-action";

const CANCELABLE = new Set<AdminJob["status"]>([
  "awaiting_demo",
  "queued",
  "running",
  "uploading",
  "pending",
  "approved",
  "processing",
]);

export function canCancel(job: AdminJob): boolean {
  return CANCELABLE.has(job.status) && !job.cancelRequested;
}

// Null when the job can be retried, otherwise the reason shown to the operator.
export function retryBlocker(job: AdminJob): string | null {
  if (job.legacyStatus || job.kind === "manual") return "Las peticiones antiguas no se pueden reintentar.";
  if (job.status !== "failed" && job.status !== "canceled") return "Solo se reintenta un trabajo fallido o cancelado.";
  if (!job.demo.present) return "La demo ya no está en el portal.";
  return null;
}

// False when JobActions would render nothing, so a row can leave the space out.
export function hasJobActions(job: AdminJob): boolean {
  return CANCELABLE.has(job.status) || retryBlocker(job) === null;
}

export interface JobActionsProps {
  job: AdminJob;
  actions: ActionRunner;
  className?: string;
}

export function JobActions({ job, actions, className }: JobActionsProps) {
  const base = `/api/admin/jobs/${job.id}`;
  const busy = actions.busyKey !== null;
  const queued = job.status === "queued";
  const retryable = retryBlocker(job) === null;
  // A boosted job that is already first gains nothing from another push.
  const alreadyFirst = job.priorityBoost > 0 && job.queue?.position === 1;

  return (
    <span className={cn("inline-flex flex-wrap items-center gap-2", className)}>
      {queued && !alreadyFirst && (
        <Button
          type="button"
          variant="outline"
          size="xs"
          disabled={busy}
          onClick={() => void actions.run(`front:${job.id}`, { url: `${base}/priority`, body: { action: "front" } })}
        >
          Subir al frente
        </Button>
      )}
      {queued && job.priorityBoost > 0 && (
        <Button
          type="button"
          variant="outline"
          size="xs"
          disabled={busy}
          onClick={() => void actions.run(`reset:${job.id}`, { url: `${base}/priority`, body: { action: "reset" } })}
        >
          Quitar prioridad
        </Button>
      )}
      {retryable && (
        <Button
          type="button"
          size="xs"
          disabled={busy}
          onClick={() => void actions.run(`retry:${job.id}`, { url: `${base}/retry` })}
        >
          Reintentar
        </Button>
      )}
      {canCancel(job) && (
        <ConfirmButton
          label="Cancelar"
          confirmLabel="Sí, cancelar"
          disabled={busy}
          busy={actions.busyKey === `cancel:${job.id}`}
          onConfirm={() => void actions.run(`cancel:${job.id}`, { url: `${base}/cancel`, body: {} })}
        />
      )}
      {job.cancelRequested && CANCELABLE.has(job.status) && (
        <span className="text-body-sm text-fg-2">Cancelación pedida, esperando al worker</span>
      )}
    </span>
  );
}
