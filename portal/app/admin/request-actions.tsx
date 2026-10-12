"use client";

import { CheckIcon } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

import type { AdminJob } from "../components/api-types";
import type { ActionRunner } from "../components/use-action";

// Same cap the portal applies to the stored reason.
const MAX_REASON_CHARS = 300;

export interface RequestActionsProps {
  job: AdminJob;
  actions: ActionRunner;
  size?: "xs" | "sm";
  className?: string;
}

// The operator's answer to a request: start the capture, or turn it down with a reason.
export function RequestActions({ job, actions, size = "xs", className }: RequestActionsProps) {
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const base = `/api/admin/jobs/${job.id}`;
  const busy = actions.busyKey !== null;

  if (rejecting) {
    return (
      <form
        className={cn("flex min-w-0 flex-wrap items-center gap-2", className)}
        onSubmit={(event) => {
          event.preventDefault();
          void actions.run(`reject:${job.id}`, { url: `${base}/reject`, body: { reason: reason.trim() } });
        }}
      >
        <Input
          className="min-w-0 flex-1 basis-56"
          aria-label="Motivo del rechazo"
          placeholder="Motivo, lo verá el usuario (opcional)"
          value={reason}
          maxLength={MAX_REASON_CHARS}
          autoFocus
          onChange={(event) => setReason(event.target.value)}
        />
        <Button type="submit" variant="destructive" size={size} disabled={busy}>
          {actions.busyKey === `reject:${job.id}` ? "Un momento" : "Rechazar"}
        </Button>
        <Button type="button" variant="outline" size={size} onClick={() => setRejecting(false)}>
          No
        </Button>
      </form>
    );
  }

  return (
    <span className={cn("inline-flex flex-wrap items-center gap-2", className)}>
      <Button
        type="button"
        size={size}
        disabled={busy}
        onClick={() => void actions.run(`approve:${job.id}`, { url: `${base}/approve` })}
      >
        <CheckIcon aria-hidden />
        {actions.busyKey === `approve:${job.id}` ? "Un momento" : "Aprobar"}
      </Button>
      <Button type="button" variant="outline-destructive" size={size} disabled={busy} onClick={() => setRejecting(true)}>
        Rechazar
      </Button>
    </span>
  );
}
