import { Badge } from "@/components/ui/badge";

import type { JobFailure } from "../components/api-types";
import { FAILURE_LABELS } from "../components/labels";

// The failure as the portal classified it, next to the sentence the user was shown.
export function FailureSummary({ failure }: { failure: JobFailure }) {
  return (
    <div className="flex flex-col gap-1.5 border-l-2 border-destructive pl-3">
      <p className="flex flex-wrap items-center gap-2">
        <Badge variant="danger" className="font-mono">
          {failure.code}
        </Badge>
        <strong className="font-semibold">{FAILURE_LABELS[failure.code]}</strong>
      </p>
      <p className="text-body-sm text-fg-2">Lo que ve el usuario: {failure.message}</p>
    </div>
  );
}

export function RawCause({ detail, open }: { detail: string; open: boolean }) {
  return (
    <details open={open}>
      <summary className="cursor-pointer rounded-sm text-body-sm text-fg-2 transition-colors duration-(--dur-fast) hover:text-fg-1">
        Causa real en el worker
      </summary>
      <pre className="mt-2 max-h-80 overflow-auto rounded-md border border-border-subtle bg-surface-0 p-3 font-mono text-[0.8125rem] leading-normal whitespace-pre-wrap wrap-anywhere">
        {detail}
      </pre>
    </details>
  );
}
