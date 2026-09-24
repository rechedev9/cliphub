'use client';

import { useEffect, useId, useState, type ReactElement } from 'react';
import { CheckCircle2, Flag, Info } from 'lucide-react';
import { useStudioTelemetry } from '@/hooks/use-studio-telemetry';
import { getDesktopSettingsBridge } from '@/lib/desktop-settings';
import {
  JOB_REPORT_CATEGORIES,
  REPORT_VIDEO_LABEL,
  reportDelivery,
  reportJob,
  reportsStayLocal,
  type JobReportCategory,
  type ReportDelivery,
} from '@/lib/api/job-report';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

/** Trigger plus dialog for a finished or failed video card. */
export function ReportVideoButton({
  jobId,
  videoTitle,
  compact = false,
}: {
  jobId: string;
  videoTitle: string;
  compact?: boolean;
}): ReactElement {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        type="button"
        size={compact ? 'icon-xs' : 'xs'}
        variant="ghost"
        aria-label={REPORT_VIDEO_LABEL}
        title={REPORT_VIDEO_LABEL}
        onClick={() => setOpen(true)}
      >
        <Flag aria-hidden />
        {compact ? null : 'Reportar un problema'}
      </Button>
      <ReportVideoDialog open={open} onOpenChange={setOpen} jobId={jobId} videoTitle={videoTitle} />
    </>
  );
}

export function ReportVideoDialog({
  open,
  onOpenChange,
  jobId,
  videoTitle,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  jobId: string;
  videoTitle: string;
}): ReactElement {
  const status = useStudioTelemetry({ fresh: true, enabled: open });
  const [category, setCategory] = useState<JobReportCategory | null>(null);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [delivered, setDelivered] = useState<ReportDelivery | null>(null);
  const descriptionId = useId();

  useEffect(() => {
    if (!open) return;
    setCategory(null);
    setSending(false);
    setError(null);
    setDelivered(null);
  }, [open]);

  const submit = (): void => {
    if (category === null || sending) return;
    setSending(true);
    setError(null);
    void reportJob(jobId, category).then(async (result) => {
      if (!result.ok) {
        setSending(false);
        setError(result.message);
        return;
      }
      // Re-read consent: the confirmation must not promise a send that cannot happen.
      const bridge = getDesktopSettingsBridge();
      const current = bridge === null ? null : await bridge.getTelemetry().catch(() => status);
      setSending(false);
      setDelivered(reportDelivery(current));
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={descriptionId}>
        <DialogHeader>
          <DialogTitle>{REPORT_VIDEO_LABEL}</DialogTitle>
          <DialogDescription id={descriptionId}>
            {delivered === null
              ? `${videoTitle}. Elige qué ha fallado: se adjunta el diagnóstico técnico de este trabajo, nunca el vídeo ni la demo.`
              : videoTitle}
          </DialogDescription>
        </DialogHeader>

        {delivered !== null ? (
          <p
            role="status"
            className={cn(
              'flex items-start gap-2 rounded-md border p-3 text-body-sm',
              delivered.tone === 'sent' ? 'border-success/40 bg-success/10 text-fg-1' : 'border-border bg-surface-3 text-fg-2',
            )}
          >
            {delivered.tone === 'sent'
              ? <CheckCircle2 aria-hidden className="mt-0.5 size-4 shrink-0 text-success" />
              : <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-fg-3" />}
            <span className="min-w-0 break-words">{delivered.text}</span>
          </p>
        ) : (
          <fieldset className="grid gap-2" disabled={sending}>
            <legend className="sr-only">Qué ha fallado</legend>
            {JOB_REPORT_CATEGORIES.map((option) => {
              const checked = category === option.value;
              return (
                <label
                  key={option.value}
                  className={cn(
                    'flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2.5 transition-colors duration-(--dur-fast)',
                    checked ? 'border-primary/70 bg-primary/10' : 'border-border bg-surface-3 hover:border-border-strong',
                  )}
                >
                  <input
                    type="radio"
                    name={`report-${jobId}`}
                    value={option.value}
                    checked={checked}
                    onChange={() => setCategory(option.value)}
                    className="mt-1 size-4 shrink-0 accent-primary"
                  />
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="text-body-sm font-semibold text-fg-1">{option.label}</span>
                    <span className="text-meta text-fg-3">{option.hint}</span>
                  </span>
                </label>
              );
            })}
          </fieldset>
        )}

        {delivered === null && reportsStayLocal(status) ? (
          <p className="flex items-start gap-2 text-body-sm text-fg-3">
            <Info aria-hidden className="mt-0.5 size-4 shrink-0" />
            Los diagnósticos están desactivados: el informe se guardará solo en este equipo hasta que los actives en Ajustes.
          </p>
        ) : null}

        {error !== null ? (
          <p role="alert" className="text-body-sm text-destructive">{error}</p>
        ) : null}

        <DialogFooter>
          {delivered !== null ? (
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cerrar</Button>
          ) : (
            <>
              <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancelar</Button>
              <Button type="button" disabled={category === null} loading={sending} loadingText="Enviando" onClick={submit}>
                Enviar informe
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
