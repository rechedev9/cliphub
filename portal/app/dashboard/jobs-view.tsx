"use client";

import { Notice, NoticeDetail } from "@/components/notice";
import { Panel } from "@/components/panel";
import { SectionHeading } from "@/components/section-heading";
import { Button } from "@/components/ui/button";

import type { StudioJob, StudioMe } from "../components/api-types";
import { ConfirmButton } from "../components/confirm-button";
import { ActionError, Empty, LoadError, Loading, Meter } from "../components/feedback";
import { estimateRangeText, formatBytes, formatDate, formatDuration } from "../components/format";
import { KIND_LABELS, STAGE_LABELS } from "../components/labels";
import { parseStudioJobs, parseStudioMe } from "../components/parse";
import { StatusPill } from "../components/status-pill";
import { useAction } from "../components/use-action";
import type { ActionRunner } from "../components/use-action";
import { usePoll } from "../components/use-poll";

const ACTIVE = new Set<StudioJob["status"]>(["awaiting_demo", "queued", "running", "uploading"]);
const MUTED = "text-body-sm text-fg-2";

function queueText(job: StudioJob, now: number): string {
  if (job.queue === null) return "En cola.";
  const position = job.queue.position;
  if (job.queue.state === "paused") return `La nube está en pausa. Tu puesto (${position}) se mantiene.`;
  if (job.queue.state === "offline") {
    return `La nube está sin conexión ahora mismo. Tu puesto (${position}) se mantiene.`;
  }
  if (job.queue.estimatedStartAt === null) return `Puesto ${position}.`;
  return `Puesto ${position} · ${estimateRangeText(job.queue.estimatedStartAt, now)}`;
}

function AccessBanner({ me }: { me: StudioMe }) {
  if (me.access === "pending") {
    return (
      <Notice tone="warning" role="status" className="mb-4">
        <div className="min-w-0">
          <strong className="font-semibold">Tu cuenta está pendiente de aprobación.</strong>
          <NoticeDetail>
            Cuando la aprobemos podrás grabar en la nube desde ClipHub Studio. Mientras tanto puedes grabar en tu PC.
          </NoticeDetail>
        </div>
      </Notice>
    );
  }
  if (me.access === "blocked") {
    return (
      <Notice tone="danger" role="status" className="mb-4">
        <strong className="font-semibold">Tu cuenta no tiene acceso a la nube de ClipHub.</strong>
      </Notice>
    );
  }
  const left = Math.max(0, me.limits.dailySeconds - me.usage.secondsLast24h - me.usage.secondsCommitted);
  return (
    <p className={`${MUTED} mb-4`}>
      {me.queue.state === "online"
        ? `${me.queue.queued} en cola ahora mismo`
        : me.queue.state === "paused"
          ? "La nube está en pausa"
          : "La nube está sin conexión"}{" "}
      · te quedan {formatDuration(left)} de grabación hoy · {me.usage.active} de {me.limits.maxActive} trabajos activos
    </p>
  );
}

interface JobCardProps {
  job: StudioJob;
  now: number;
  actions: ActionRunner;
}

function JobCard({ job, now, actions }: JobCardProps) {
  const base = `/api/studio/jobs/${encodeURIComponent(job.id)}`;
  return (
    <li>
      <Panel tone={job.status === "running" ? "stream" : "neutral"}>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <StatusPill status={job.status} stage={job.stage} />
          <strong className="min-w-0 font-semibold wrap-anywhere">{job.title ?? "Sin título"}</strong>
          <span className={MUTED}>{KIND_LABELS[job.kind]}</span>
        </div>

        {job.status === "awaiting_demo" && (
          <p className="text-fg-2">ClipHub Studio está subiendo la demo. Déjalo abierto hasta que termine.</p>
        )}

        {job.status === "queued" && (
          <div className="flex flex-col gap-1">
            <p>{queueText(job, now)}</p>
            {job.queue?.state === "online" && job.queue.estimatedStartAt !== null && (
              <p className={MUTED}>Es una estimación y puede moverse según lo que haya delante.</p>
            )}
          </div>
        )}

        {job.status === "running" && (
          <div className="flex flex-col gap-1.5">
            <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5">
              <span>{job.stage === null ? "Preparando" : STAGE_LABELS[job.stage]}</span>
              <span className="font-mono tabular-nums">
                {job.progressPercent === null ? "" : `${job.progressPercent} %`}
              </span>
            </div>
            <Meter percent={job.progressPercent ?? 0} tone="live" label="Progreso de la grabación" />
          </div>
        )}

        {job.status === "uploading" && (
          <p className="text-fg-2">El vídeo ya está montado y se está subiendo. Aparecerá aquí en unos minutos.</p>
        )}

        {job.status === "done" && job.artifacts.length === 0 && (
          <p className="text-fg-2">
            Los archivos ya no están disponibles aquí: se borran pasado el plazo de conservación.
          </p>
        )}
        {job.status === "done" && job.artifacts.length > 0 && (
          <ul className="flex flex-col gap-2.5">
            {job.artifacts.map((artifact) => (
              <li key={artifact.id} className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
                <Button asChild size="sm" className="max-w-full">
                  <a href={`${base}/artifacts/${encodeURIComponent(artifact.id)}`}>
                    <span className="min-w-0 truncate">Descargar {artifact.name}</span>
                  </a>
                </Button>
                <span className={MUTED}>
                  {formatBytes(artifact.sizeBytes)} · disponible hasta el {formatDate(artifact.expiresAt)}
                </span>
              </li>
            ))}
          </ul>
        )}

        {job.status === "failed" && (
          <p className="text-destructive">
            {job.failure === null || job.failure.message === ""
              ? "No se pudo completar este trabajo."
              : job.failure.message}
          </p>
        )}
        {job.status === "canceled" && <p className="text-fg-2">Este trabajo se canceló.</p>}

        {ACTIVE.has(job.status) && (
          <div className="flex flex-wrap items-center gap-2">
            {job.cancelRequested ? (
              <span className={MUTED}>Cancelando. Puede tardar unos segundos.</span>
            ) : (
              <ConfirmButton
                label="Cancelar"
                confirmLabel="Sí, cancelar"
                size="sm"
                disabled={actions.busyKey !== null}
                busy={actions.busyKey === `cancel:${job.id}`}
                onConfirm={() => void actions.run(`cancel:${job.id}`, { url: `${base}/cancel` })}
              />
            )}
          </div>
        )}
      </Panel>
    </li>
  );
}

export function JobsView() {
  const jobsPoll = usePoll({ url: "/api/studio/jobs", intervalMs: 5000, parse: parseStudioJobs });
  const mePoll = usePoll({ url: "/api/studio/me", intervalMs: 30000, parse: parseStudioMe });
  const actions = useAction(jobsPoll.refresh);
  const jobs = jobsPoll.data?.jobs ?? null;
  const now = Date.now();

  return (
    <section>
      <SectionHeading title="Mis trabajos en la nube" />
      {mePoll.data !== null && <AccessBanner me={mePoll.data} />}

      {jobsPoll.error !== null && (
        <LoadError error={jobsPoll.error} onRetry={jobsPoll.refresh} staleSince={jobsPoll.loadedAt} />
      )}
      <ActionError message={actions.error} />
      {jobs === null && jobsPoll.error === null && <Loading>Cargando tus trabajos.</Loading>}

      {jobs !== null && jobs.length === 0 && (
        <Empty>
          <p className="text-fg-1">Todavía no tienes ningún trabajo en la nube.</p>
          {mePoll.data?.access !== "blocked" && (
            <p>
              Los trabajos se crean desde ClipHub Studio: abre una demo, elige tus jugadas y, en «Dónde grabar», marca
              «Nube ClipHub».
            </p>
          )}
        </Empty>
      )}

      <ul className="flex flex-col gap-3">
        {(jobs ?? []).map((job) => (
          <JobCard key={job.id} job={job} now={now} actions={actions} />
        ))}
      </ul>

      {jobs !== null && jobs.length > 0 && (
        <p className={`${MUTED} mt-4 max-w-[68ch]`}>
          Los trabajos se crean desde ClipHub Studio. Studio también descarga los vídeos por ti; estos enlaces son por
          si quieres bajarlos desde otro equipo.
        </p>
      )}
    </section>
  );
}
