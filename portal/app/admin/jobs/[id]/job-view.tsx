"use client";

import { Facts, Fact } from "@/components/facts";
import { Notice } from "@/components/notice";
import { Panel } from "@/components/panel";
import { SectionHeading } from "@/components/section-heading";
import { TextLink } from "@/components/text-link";

import type { AdminEvent, AdminJob, AdminWorker, JobSpecSummary } from "../../../components/api-types";
import { ActionError, Empty, LoadError, Loading, Meter } from "../../../components/feedback";
import { captureSeconds, formatBytes, formatDateTime, formatDuration } from "../../../components/format";
import { eventDetailText, eventLabel, KIND_LABELS, STAGE_LABELS } from "../../../components/labels";
import { parseAdminJobDetail, parseAdminWorkers } from "../../../components/parse";
import { jobTitle, userLabel } from "../../../components/queue-summary";
import { StatusPill } from "../../../components/status-pill";
import { useAction } from "../../../components/use-action";
import { usePoll } from "../../../components/use-poll";
import { FailureSummary, RawCause } from "../../failure";
import { hasJobActions, JobActions, retryBlocker } from "../../job-actions";
import { Timeline, TimelineItem } from "../../timeline";

const MUTED = "text-body-sm text-fg-2";

function Summary({ job, workers }: { job: AdminJob; workers: AdminWorker[] }) {
  const worker = workers.find((candidate) => candidate.id === job.workerId);
  const rows = [
    { label: "Usuario", value: `${userLabel(job.user)}${job.user.email === null ? "" : ` (${job.user.email})`}` },
    { label: "Tipo", value: KIND_LABELS[job.kind] },
    { label: "Creado", value: formatDateTime(job.createdAt) },
    { label: "Empezó", value: job.startedAt === null ? "Todavía no" : formatDateTime(job.startedAt) },
    { label: "Terminó", value: job.finishedAt === null ? "Todavía no" : formatDateTime(job.finishedAt) },
    { label: "Intento", value: `${job.attempt} de ${job.maxAttempts}` },
    {
      label: "Estimación",
      value: job.estimatedSeconds === null ? "Sin datos" : formatDuration(job.estimatedSeconds),
    },
    { label: "Tiempo de máquina", value: formatDuration(job.machineSeconds) },
    { label: "Esperó en cola", value: job.waitedSeconds === null ? "Sin datos" : formatDuration(job.waitedSeconds) },
    { label: "Prioridad", value: job.priorityBoost > 0 ? `Subido al frente (${job.priorityBoost})` : "Normal" },
    { label: "Job local en el worker", value: job.localJobId ?? "Ninguno" },
    { label: "Worker", value: worker?.name ?? job.workerId ?? "Ninguno" },
    {
      label: "Demo",
      value: `${job.demo.fileName ?? "demo.dem"}${
        job.demo.sizeBytes === null ? "" : `, ${formatBytes(job.demo.sizeBytes)}`
      }${job.demo.present ? "" : ", ya borrada del portal"}`,
    },
  ];
  return (
    <Facts wide>
      {rows.map((row) => (
        <Fact key={row.label} label={row.label}>
          {row.value}
        </Fact>
      ))}
    </Facts>
  );
}

function SpecSummary({ spec }: { spec: JobSpecSummary }) {
  return (
    <Facts wide>
      <Fact label="SteamID del jugador" className="font-mono">
        {spec.targetSteamId}
      </Fact>
      <Fact label="Preset" className="font-mono">
        {spec.preset ?? "Sin preset"}
      </Fact>
      <Fact label="Ventanas">{spec.windows.length}</Fact>
      <Fact label="Captura">{formatDuration(captureSeconds(spec))}</Fact>
      <Fact label="Versión del plan" className="font-mono">
        {spec.planSchema ?? "No indicada"}
      </Fact>
    </Facts>
  );
}

function JobTimeline({ events }: { events: AdminEvent[] }) {
  if (events.length === 0) return <p className={MUTED}>Sin eventos registrados.</p>;
  return (
    <Timeline>
      {events.map((event) => {
        const detail = eventDetailText(event.type, event.detail);
        return (
          <TimelineItem key={event.id} at={event.at} actor={event.actor}>
            <strong className="font-semibold">{eventLabel(event.type)}</strong>
            {detail !== null && <span className="text-fg-2"> · {detail}</span>}
          </TimelineItem>
        );
      })}
    </Timeline>
  );
}

export function JobView({ id }: { id: string }) {
  const poll = usePoll({
    url: `/api/admin/jobs/${encodeURIComponent(id)}`,
    intervalMs: 5000,
    parse: parseAdminJobDetail,
  });
  // Only used to show the worker by name; the page works without it.
  const workersPoll = usePoll({ url: "/api/admin/workers", intervalMs: 60000, parse: parseAdminWorkers });
  const actions = useAction(poll.refresh);
  const detail = poll.data;

  if (detail === null) {
    if (poll.error?.code === "not_found") {
      return (
        <Empty>
          <p>
            Este trabajo no existe. <TextLink href="/admin/history">Volver al historial</TextLink>.
          </p>
        </Empty>
      );
    }
    if (poll.error !== null) return <LoadError error={poll.error} onRetry={poll.refresh} staleSince={null} />;
    return <Loading>Cargando el trabajo.</Loading>;
  }

  const { job, spec, events } = detail;
  const legacy = job.legacyStatus || job.kind === "manual";
  const ended = job.status === "failed" || job.status === "canceled";
  const blocker = ended && !legacy ? retryBlocker(job) : null;
  const videos = job.artifacts.filter((artifact) => artifact.kind === "video");

  return (
    <>
      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      <ActionError message={actions.error} />

      <Panel>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <StatusPill status={job.status} stage={job.stage} />
          <h2 className="min-w-0 text-title font-semibold wrap-anywhere">{jobTitle(job)}</h2>
        </div>

        {legacy && (
          <Notice tone="info">Petición del sistema antiguo (subida manual). Solo se puede consultar y cancelar.</Notice>
        )}

        {job.status === "running" && (
          <div className="flex flex-col gap-1.5">
            <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5">
              <span>
                {job.stage === null ? "Preparando" : STAGE_LABELS[job.stage]}
                {job.progressDetail !== null && (
                  <span className="font-mono text-body-sm text-fg-2"> · {job.progressDetail}</span>
                )}
              </span>
              <span className="font-mono tabular-nums">
                {job.progressPercent === null ? "" : `${job.progressPercent} %`}
              </span>
            </div>
            <Meter percent={job.progressPercent ?? 0} tone="live" label="Progreso del trabajo" />
          </div>
        )}

        {job.status === "queued" && job.queue !== null && (
          <p>
            Puesto {job.queue.position} en la cola
            {job.queue.estimatedStartAt === null
              ? ", sin estimación porque la nube no está en línea."
              : `, empieza hacia las ${formatDateTime(job.queue.estimatedStartAt)}.`}
          </p>
        )}

        {job.failure !== null && <FailureSummary failure={job.failure} />}
        {job.failureDetail !== null && <RawCause detail={job.failureDetail} open={job.status === "failed"} />}
        {job.status === "canceled" && job.canceledBy !== null && (
          <p className={MUTED}>Cancelado por {job.canceledBy === "admin" ? "un operador" : "el usuario"}.</p>
        )}

        {(hasJobActions(job) || blocker !== null) && (
          <div className="flex flex-wrap items-center gap-2">
            {hasJobActions(job) && <JobActions job={job} actions={actions} />}
            {blocker !== null && <span className={MUTED}>No se puede reintentar: {blocker}</span>}
          </div>
        )}
      </Panel>

      <SectionHeading title="Resumen" />
      <Panel>
        <Summary job={job} workers={workersPoll.data?.workers ?? []} />
      </Panel>

      {!legacy && (
        <>
          <SectionHeading title="Plan de captura" />
          <Panel>
            {spec === null ? <p className={MUTED}>Este trabajo no tiene plan guardado.</p> : <SpecSummary spec={spec} />}
          </Panel>
        </>
      )}

      <SectionHeading title="Resultados" count={job.artifacts.length} />
      {job.artifacts.length === 0 && (
        <Empty>
          {job.status === "done"
            ? "Los archivos ya se han borrado del portal."
            : ended || job.status === "rejected"
              ? "Este trabajo no dejó resultados."
              : "Todavía no hay resultados subidos."}
        </Empty>
      )}
      <div className="grid gap-4 sm:grid-cols-[repeat(auto-fill,minmax(20rem,1fr))]">
        {videos.map((artifact) => (
          <Panel key={artifact.id}>
            <p>
              <strong className="font-semibold wrap-anywhere">{artifact.name}</strong>{" "}
              <span className={MUTED}>
                {artifact.variant} · {formatBytes(artifact.sizeBytes)} · se borra el{" "}
                {formatDateTime(artifact.expiresAt)}
              </span>
            </p>
            <video
              className="max-h-[36rem] w-full rounded-md border border-border bg-surface-0"
              controls
              preload="metadata"
              src={`/api/admin/jobs/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(artifact.id)}`}
            />
          </Panel>
        ))}
      </div>

      <SectionHeading title="Cronología" />
      <Panel>
        <JobTimeline events={events} />
      </Panel>
    </>
  );
}
