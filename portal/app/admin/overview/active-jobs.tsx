import { Panel } from "@/components/panel";
import { TextLink } from "@/components/text-link";

import type { AdminJob, AdminWorker } from "../../components/api-types";
import { Meter } from "../../components/feedback";
import { formatDuration } from "../../components/format";
import { KIND_LABELS, STAGE_LABELS } from "../../components/labels";
import { jobTitle, leaseOutlook, userLabel } from "../../components/queue-summary";
import type { ActionRunner } from "../../components/use-action";
import { JobActions } from "../job-actions";
import { MUTED, SPLIT_ROW } from "./styles";

export interface ActiveJobsProps {
  jobs: AdminJob[];
  workers: AdminWorker[];
  now: number;
  actions: ActionRunner;
  mode: "running" | "uploading";
}

export function ActiveJobs({ jobs, workers, now, actions, mode }: ActiveJobsProps) {
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
