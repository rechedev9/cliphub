'use client';

import { useState, type ReactElement } from 'react';
import { usePathname } from 'next/navigation';
import { ClipboardCopy } from 'lucide-react';
import { useStudioTelemetry } from '@/hooks/use-studio-telemetry';
import { writeClipboardText } from '@/lib/clipboard-write';
import { diagnosticLines, diagnosticText, type DiagnosticLine } from '@/lib/diagnostic-summary';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

type DiagnosticContext = {
  jobId?: string;
  digest?: string;
  message?: string;
};

function useDiagnosticLines(context: DiagnosticContext): DiagnosticLine[] {
  const status = useStudioTelemetry();
  const pathname = usePathname();
  return diagnosticLines({
    supportCode: status?.supportCode,
    sessionId: status?.sessionId,
    jobId: context.jobId,
    digest: context.digest,
    route: pathname ?? undefined,
    message: context.message,
  });
}

function useCopy(lines: readonly DiagnosticLine[]): { copied: boolean; failed: boolean; copy: () => void } {
  const [copied, setCopied] = useState(false);
  const [failed, setFailed] = useState(false);
  const copy = (): void => {
    setFailed(false);
    void writeClipboardText(diagnosticText(lines))
      .then(() => {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 2_500);
      })
      .catch(() => setFailed(true));
  };
  return { copied, failed, copy };
}

/**
 * Failure-card line: one action that copies every identifier support needs.
 * The card column is too narrow to show the 27-character support code without
 * breaking it mid-group, so it rides in the tooltip and the copied text.
 */
export function DiagnosticInline({ jobId, className }: { jobId?: string; className?: string }): ReactElement {
  const lines = useDiagnosticLines({ jobId });
  const { copied, failed, copy } = useCopy(lines);
  const supportCode = lines.find((line) => line.label === 'Código de soporte')?.value;
  return (
    <span className={cn('flex flex-wrap items-center gap-x-2 gap-y-1 text-meta text-fg-3', className)}>
      <button
        type="button"
        onClick={copy}
        title={supportCode ? `Código de soporte ${supportCode}` : undefined}
        className="inline-flex h-7 items-center gap-1 rounded-sm text-fg-2 underline decoration-border-strong underline-offset-4 transition-colors duration-(--dur-fast) hover:text-primary hover:decoration-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      >
        <ClipboardCopy aria-hidden className="size-3" />
        {copied ? 'Diagnóstico copiado' : 'Copiar diagnóstico'}
      </button>
      {failed ? <span role="alert" className="text-destructive">No se pudo copiar.</span> : null}
    </span>
  );
}

function copyLabel(copied: boolean, failed: boolean): string {
  if (failed) return 'No se pudo copiar';
  return copied ? 'Diagnóstico copiado' : 'Copiar diagnóstico';
}

/** Row action for a failed partida, whose header is itself a button. */
export function CopyDiagnosticButton({ jobId }: { jobId?: string }): ReactElement {
  const lines = useDiagnosticLines({ jobId });
  const { copied, failed, copy } = useCopy(lines);
  return (
    <Button type="button" size="sm" variant="ghost" onClick={copy} aria-live="polite">
      <ClipboardCopy aria-hidden />
      {copyLabel(copied, failed)}
    </Button>
  );
}

/** Error-boundary block: every identifier visible, plus the copy action. */
export function DiagnosticBlock({ digest, message, className }: { digest?: string; message?: string; className?: string }): ReactElement {
  const lines = useDiagnosticLines({ digest, message });
  const { copied, failed, copy } = useCopy(lines);
  return (
    <div className={cn('flex flex-col gap-3 rounded-md border border-border bg-surface-0 p-4', className)}>
      <dl className="grid gap-x-4 gap-y-1.5 text-body-sm sm:grid-cols-[auto_minmax(0,1fr)]">
        {lines.map((line) => (
          <div key={line.label} className="contents">
            <dt className="text-fg-3">{line.label}</dt>
            <dd className="min-w-0 break-all text-fg-1 tabular-nums">{line.value}</dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" size="sm" variant="outline" onClick={copy}>
          <ClipboardCopy aria-hidden />
          {copied ? 'Diagnóstico copiado' : 'Copiar diagnóstico'}
        </Button>
        {failed ? <span role="alert" className="text-body-sm text-destructive">No se pudo copiar el diagnóstico.</span> : null}
      </div>
    </div>
  );
}
