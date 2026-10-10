"use client";

import { Notice } from "@/components/notice";
import { Panel } from "@/components/panel";
import { SectionHeading } from "@/components/section-heading";
import { StatusDot, TONE_TEXT } from "@/components/status-dot";
import { TextLink } from "@/components/text-link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

import type { AdminJob, AdminOverview, AdminStats, AdminWorker } from "../components/api-types";
import { ActionError, Empty, LoadError, Loading, Meter } from "../components/feedback";
import { formatBytes, formatClock, formatDuration, formatRelative, percentOf } from "../components/format";
import { FAILURE_LABELS, KIND_LABELS, STAGE_LABELS } from "../components/labels";
import { parseAdminJobs, parseOverview } from "../components/parse";
import {
  causeHeadline,
  jobTitle,
  leaseOutlook,
  queueSummary,
  recentFailures,
  userLabel,
} from "../components/queue-summary";
import type { SummaryCell } from "../components/queue-summary";
import { useAction } from "../components/use-action";
import type { ActionRunner } from "../components/use-action";
import { usePoll } from "../components/use-poll";
import { JobActions } from "./job-actions";
import { WorkerCard } from "./worker-card";

const LOW_STORAGE_BYTES = 15 * 1024 ** 3;
const FAILED_HISTORY = "/admin/history?status=failed";
const MAX_RECENT_FAILURES = 3;

// A ruled grid: the 1px gap shows the container colour as hairlines.
const RULED = "grid gap-px overflow-hidden rounded-lg border border-border bg-border-subtle shadow-sm";
const CELL_LABEL = "font-mono text-meta text-fg-3 uppercase";
const SPLIT_ROW = "flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5";
const MUTED = "text-body-sm text-fg-2";

// Below lg the queue table becomes one block per job, so actions stay in reach.
const STACKED_ROW = "max-lg:block max-lg:px-4 max-lg:py-3";
const STACKED_CELL = [
  "max-lg:flex max-lg:items-baseline max-lg:justify-between max-lg:gap-4 max-lg:px-0 max-lg:py-1 max-lg:text-right",
  "max-lg:before:shrink-0 max-lg:before:text-left max-lg:before:font-mono max-lg:before:text-meta",
  "max-lg:before:text-fg-3 max-lg:before:uppercase max-lg:before:content-[attr(data-label)]",
].join(" ");

function Summary({ cells }: { cells: SummaryCell[] }) {
  return (
    <section className={cn(RULED, "mb-5 grid-cols-2 lg:grid-cols-4")} aria-label="Estado de la nube">
      {cells.map((cell) => {
        const body = (
          <>
            <span className={cn(CELL_LABEL, "flex items-center gap-2 wrap-anywhere")}>
              <StatusDot tone={cell.tone === "plain" ? "off" : cell.tone} className="size-2" />
              {cell.label}
            </span>
            <span
              className={cn(
                "text-body-lg leading-tight font-semibold wrap-anywhere sm:text-title",
                cell.tone !== "plain" && TONE_TEXT[cell.tone],
              )}
            >
              {cell.value}
            </span>
            {cell.detail !== null && <span className={cn(MUTED, "wrap-anywhere")}>{cell.detail}</span>}
          </>
        );
        const className = cn(
          "flex min-w-0 flex-col gap-1.5 bg-surface-2 px-3.5 py-3 text-fg-1 sm:px-4 sm:py-3.5",
          // The one cell that is in trouble is tinted, so it is found before it is read.
          cell.tone === "bad" && "bg-[color-mix(in_oklch,var(--destructive)_9%,var(--surface-2))]",
        );
        return cell.href === null ? (
          <div key={cell.key} className={className}>
            {body}
          </div>
        ) : (
          <a
            key={cell.key}
            className={cn(className, "transition-colors duration-(--dur-fast) hover:bg-surface-3")}
            href={cell.href}
          >
            {body}
          </a>
        );
      })}
    </section>
  );
}

function Stats({ stats }: { stats: AdminStats }) {
  const items = [
    { label: "En cola", value: String(stats.queued), tone: "", href: null },
    { label: "Hechos en 24 h", value: String(stats.done24h), tone: "text-success", href: "/admin/history?status=done" },
    {
      label: "Fallidos en 24 h",
      value: String(stats.failed24h),
      tone: stats.failed24h > 0 ? "text-destructive" : "",
      href: stats.failed24h > 0 ? FAILED_HISTORY : null,
    },
    { label: "Cancelados en 24 h", value: String(stats.canceled24h), tone: "", href: null },
    {
      label: "Espera mediana en 24 h",
      value: stats.medianWaitSeconds24h === null ? "Sin datos" : formatDuration(stats.medianWaitSeconds24h),
      tone: "",
      href: null,
    },
    { label: "Máquina en 24 h", value: formatDuration(stats.machineSeconds24h), tone: "", href: null },
  ];
  return (
    <dl className={cn(RULED, "mb-4 grid-cols-2 sm:grid-cols-3 lg:grid-cols-6")}>
      {items.map((item) => (
        <div key={item.label} className="flex min-w-0 flex-col justify-between gap-1.5 bg-surface-2 px-3.5 py-3 sm:px-4">
          <dt className={cn(CELL_LABEL, "tracking-[0.08em]")}>{item.label}</dt>
          <dd className={cn("text-title font-semibold tabular-nums", item.tone)}>
            {item.href === null ? (
              item.value
            ) : (
              <a className="underline decoration-current/35 underline-offset-[0.2em] hover:decoration-current" href={item.href}>
                {item.value}
              </a>
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function Storage({ storage }: { storage: AdminStats["storage"] }) {
  const low = storage.freeBytes < LOW_STORAGE_BYTES;
  return (
    <Panel tone={low ? "danger" : "neutral"} className="gap-2">
      <div className={SPLIT_ROW}>
        <h3 className="font-semibold">Almacenamiento del portal</h3>
        <span className={cn("font-mono text-body-sm tabular-nums", low && "text-destructive")}>
          {formatBytes(storage.freeBytes)} libres de {formatBytes(storage.totalBytes)}
        </span>
      </div>
      <Meter
        percent={100 - percentOf(storage.freeBytes, storage.totalBytes)}
        tone={low ? "bad" : "primary"}
        label="Espacio usado en el portal"
      />
      <p className={MUTED}>
        Demos {formatBytes(storage.demoBytes)} · Resultados {formatBytes(storage.artifactBytes)}
        {low && " · Con menos de 10 GB libres el portal deja de aceptar trabajos."}
      </p>
    </Panel>
  );
}

function RecentFailures({ jobs, now }: { jobs: AdminJob[]; now: number }) {
  return (
    <ul className="flex flex-col gap-3">
      {jobs.map((job) => {
        const cause = causeHeadline(job.failureDetail);
        return (
          <li key={job.id}>
            <Panel className="gap-1.5">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
                {job.failure !== null && (
                  <Badge variant="danger" className="font-mono uppercase">
                    {FAILURE_LABELS[job.failure.code]}
                  </Badge>
                )}
                <TextLink className="min-w-0 font-semibold wrap-anywhere" href={`/admin/jobs/${job.id}`}>
                  {jobTitle(job)}
                </TextLink>
                <span className={MUTED}>
                  {userLabel(job.user)}
                  {job.finishedAt !== null && ` · ${formatRelative(job.finishedAt, now)}`}
                </span>
              </div>
              {cause !== null && <p className="font-mono text-body-sm text-fg-2 wrap-anywhere">{cause}</p>}
            </Panel>
          </li>
        );
      })}
    </ul>
  );
}

interface ActiveJobsProps {
  jobs: AdminJob[];
  workers: AdminWorker[];
  now: number;
  actions: ActionRunner;
  mode: "running" | "uploading";
}

function ActiveJobs({ jobs, workers, now, actions, mode }: ActiveJobsProps) {
  return (
    <ul className="flex flex-col gap-3">
      {jobs.map((job) => {
        const stage = mode === "uploading" ? "Subiendo" : job.stage === null ? "Preparando" : STAGE_LABELS[job.stage];
        // Revoked workers are not in the overview, so a job of theirs has no owner here.
        const owner = workers.find((worker) => worker.id === job.workerId);
        const silent = owner === undefined || !owner.online;
        return (
          <li key={job.id}>
            <Panel className="grid gap-x-6 gap-y-3 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_auto] lg:items-center">
              <div className="flex min-w-0 flex-col gap-1">
                <TextLink className="font-semibold wrap-anywhere" href={`/admin/jobs/${job.id}`}>
                  {jobTitle(job)}
                </TextLink>
                <span className={MUTED}>
                  {userLabel(job.user)} · {KIND_LABELS[job.kind]}
                  {owner !== undefined && workers.length > 1 && ` · en ${owner.name}`} · intento {job.attempt} de{" "}
                  {job.maxAttempts}
                  {job.startedAt !== null && ` · lleva ${formatDuration((now - job.startedAt) / 1000)}`}
                  {mode === "running" &&
                    job.estimatedSeconds !== null &&
                    ` de unos ${formatDuration(job.estimatedSeconds)}`}
                </span>
                {silent && (
                  <span className="text-body-sm text-warning">
                    {owner === undefined ? "Su worker ya no está disponible." : `Sin señal de ${owner.name}.`}{" "}
                    {leaseOutlook(job, now)}
                  </span>
                )}
              </div>
              <div className="flex min-w-0 flex-col gap-1.5">
                <div className={SPLIT_ROW}>
                  <span>
                    {stage}
                    {job.progressDetail !== null && (
                      <span className="font-mono text-body-sm text-fg-2"> · {job.progressDetail}</span>
                    )}
                  </span>
                  <span className="font-mono tabular-nums">
                    {job.progressPercent === null ? "" : `${job.progressPercent} %`}
                  </span>
                </div>
                <Meter
                  percent={job.progressPercent ?? 0}
                  tone={silent ? "off" : mode === "running" ? "live" : "primary"}
                  label={`Progreso de ${jobTitle(job)}`}
                />
              </div>
              <JobActions job={job} actions={actions} />
            </Panel>
          </li>
        );
      })}
    </ul>
  );
}

function StartCell({ job }: { job: AdminJob }) {
  if (job.queue !== null && job.queue.estimatedStartAt !== null) return formatClock(job.queue.estimatedStartAt);
  // With a worker ready, a job without a start time is one the scheduler skips.
  if (job.queue?.state === "online") {
    return <span className="text-warning">Retenido: usuario bloqueado o tipo desactivado</span>;
  }
  return <span className="whitespace-nowrap text-fg-3">Sin estimación</span>;
}

interface QueueTableProps {
  jobs: AdminJob[];
  actions: ActionRunner;
}

function QueueTable({ jobs, actions }: QueueTableProps) {
  return (
    <div className="studio-panel overflow-hidden">
      <Table className="max-lg:block">
        <TableHeader className="max-lg:hidden">
          <TableRow>
            <TableHead scope="col">Puesto</TableHead>
            <TableHead scope="col">Usuario</TableHead>
            <TableHead scope="col">Título</TableHead>
            <TableHead scope="col">Tipo</TableHead>
            <TableHead scope="col">Estimación</TableHead>
            <TableHead scope="col">Esperando</TableHead>
            <TableHead scope="col">Empieza</TableHead>
            <TableHead scope="col">Intentos</TableHead>
            <TableHead scope="col">Acciones</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className="max-lg:block">
          {jobs.map((job, index) => (
            <TableRow key={job.id} className={STACKED_ROW}>
              <TableCell className={cn(STACKED_CELL, "font-mono whitespace-nowrap tabular-nums")} data-label="Puesto">
                <span className="inline-flex items-center gap-2">
                  {job.queue?.position ?? index + 1}
                  {job.priorityBoost > 0 && (
                    <Badge variant="warning" className="font-mono uppercase">
                      Prioridad
                    </Badge>
                  )}
                </span>
              </TableCell>
              <TableCell className={cn(STACKED_CELL, "wrap-anywhere")} data-label="Usuario">
                {userLabel(job.user)}
              </TableCell>
              <TableCell className={cn(STACKED_CELL, "wrap-anywhere lg:min-w-48")} data-label="Título">
                <TextLink href={`/admin/jobs/${job.id}`}>{jobTitle(job)}</TextLink>
              </TableCell>
              <TableCell className={STACKED_CELL} data-label="Tipo">
                {KIND_LABELS[job.kind]}
              </TableCell>
              <TableCell className={cn(STACKED_CELL, "whitespace-nowrap")} data-label="Estimación">
                {job.estimatedSeconds === null ? "Sin datos" : formatDuration(job.estimatedSeconds)}
              </TableCell>
              <TableCell className={cn(STACKED_CELL, "whitespace-nowrap")} data-label="Esperando">
                {job.waitedSeconds === null ? "Sin datos" : formatDuration(job.waitedSeconds)}
              </TableCell>
              <TableCell className={STACKED_CELL} data-label="Empieza">
                <StartCell job={job} />
              </TableCell>
              <TableCell className={cn(STACKED_CELL, "font-mono whitespace-nowrap tabular-nums")} data-label="Intentos">
                {job.attempt} de {job.maxAttempts}
              </TableCell>
              <TableCell className={STACKED_CELL} data-label="Acciones">
                <JobActions job={job} actions={actions} className="max-lg:justify-end lg:flex-nowrap" />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function findJob(overview: AdminOverview, id: string | null): AdminJob | null {
  if (id === null) return null;
  return overview.running.find((job) => job.id === id) ?? overview.uploading.find((job) => job.id === id) ?? null;
}

export function QueueView() {
  const poll = usePoll({ url: "/api/admin/overview", intervalMs: 3000, parse: parseOverview });
  // A failed load here only hides the recent failures; the counter still comes from the overview.
  const failedPoll = usePoll({ url: "/api/admin/jobs?status=failed&limit=5", intervalMs: 15000, parse: parseAdminJobs });
  const actions = useAction(poll.refresh);
  const overview = poll.data;
  const failures = overview === null ? [] : recentFailures(failedPoll.data?.jobs ?? [], overview.now);
  const lowStorage = overview !== null && overview.stats.storage.freeBytes < LOW_STORAGE_BYTES;

  return (
    <>
      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      <ActionError message={actions.error} />
      {overview === null && poll.error === null && <Loading>Cargando el estado de la cola.</Loading>}

      {overview !== null && (
        <>
          <Summary cells={queueSummary(overview, failures)} />

          {overview.pendingUsers > 0 && (
            <Notice tone="info" role="status" className="mb-4">
              <strong className="font-semibold">
                {overview.pendingUsers === 1
                  ? "1 persona espera acceso a la nube"
                  : `${overview.pendingUsers} personas esperan acceso a la nube`}
              </strong>
              <Button asChild variant="outline-primary" size="xs">
                <a href="/admin/users?access=pending">Revisar en Usuarios</a>
              </Button>
            </Notice>
          )}
          {lowStorage && <Storage storage={overview.stats.storage} />}

          <SectionHeading title="Workers" />
          {overview.workers.length === 0 ? (
            <Empty>
              <p>
                No hay ningún worker registrado, así que nadie graba la cola.{" "}
                <TextLink href="/admin/workers">Crear uno en Workers</TextLink>.
              </p>
            </Empty>
          ) : (
            <div className="grid gap-4 md:grid-cols-[repeat(auto-fit,minmax(24rem,1fr))]">
              {overview.workers.map((worker) => (
                <WorkerCard
                  key={worker.id}
                  worker={worker}
                  currentJob={findJob(overview, worker.currentJobId)}
                  now={overview.now}
                  actions={actions}
                />
              ))}
            </div>
          )}

          {failures.length > 0 && (
            <>
              <SectionHeading title="Fallos recientes" count={overview.stats.failed24h}>
                <TextLink href={FAILED_HISTORY}>Ver todos</TextLink>
              </SectionHeading>
              <RecentFailures jobs={failures.slice(0, MAX_RECENT_FAILURES)} now={overview.now} />
            </>
          )}

          <SectionHeading title="En curso" count={overview.running.length} />
          {overview.running.length === 0 ? (
            <Empty>Nada en curso ahora mismo.</Empty>
          ) : (
            <ActiveJobs
              jobs={overview.running}
              workers={overview.workers}
              now={overview.now}
              actions={actions}
              mode="running"
            />
          )}

          {overview.uploading.length > 0 && (
            <>
              <SectionHeading title="Subiendo resultados" count={overview.uploading.length} />
              <ActiveJobs
                jobs={overview.uploading}
                workers={overview.workers}
                now={overview.now}
                actions={actions}
                mode="uploading"
              />
            </>
          )}

          <SectionHeading title="En cola" count={overview.queue.length} />
          {overview.queue.length === 0 ? (
            <Empty>La cola está vacía.</Empty>
          ) : (
            <QueueTable jobs={overview.queue} actions={actions} />
          )}

          <SectionHeading title="Últimas 24 horas" />
          <Stats stats={overview.stats} />
          {!lowStorage && <Storage storage={overview.stats.storage} />}
        </>
      )}
    </>
  );
}
