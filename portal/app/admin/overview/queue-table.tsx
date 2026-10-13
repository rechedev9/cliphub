import { TextLink } from "@/components/text-link";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

import type { AdminJob } from "../../components/api-types";
import { formatClock, formatDuration } from "../../components/format";
import { KIND_LABELS } from "../../components/labels";
import { jobTitle, userLabel } from "../../components/queue-summary";
import type { ActionRunner } from "../../components/use-action";
import { JobActions } from "../job-actions";

// Below lg the queue table becomes one block per job, so actions stay in reach.
const STACKED_ROW = "max-lg:block max-lg:px-4 max-lg:py-3";
const STACKED_CELL = [
  "max-lg:flex max-lg:items-baseline max-lg:justify-between max-lg:gap-4 max-lg:px-0 max-lg:py-1 max-lg:text-right",
  "max-lg:before:shrink-0 max-lg:before:text-left max-lg:before:font-mono max-lg:before:text-meta",
  "max-lg:before:text-fg-3 max-lg:before:uppercase max-lg:before:content-[attr(data-label)]",
].join(" ");

function StartCell({ job }: { job: AdminJob }) {
  if (job.queue !== null && job.queue.estimatedStartAt !== null) return formatClock(job.queue.estimatedStartAt);
  // With a worker ready, a job without a start time is one the scheduler skips.
  if (job.queue?.state === "online") {
    return <span className="text-warning">Retenido: usuario bloqueado o tipo desactivado</span>;
  }
  return <span className="whitespace-nowrap text-fg-3">Sin estimación</span>;
}

export interface QueueTableProps {
  jobs: AdminJob[];
  actions: ActionRunner;
}

export function QueueTable({ jobs, actions }: QueueTableProps) {
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
