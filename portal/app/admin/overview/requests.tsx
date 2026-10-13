import { Panel } from "@/components/panel";
import { TextLink } from "@/components/text-link";

import type { AdminJob } from "../../components/api-types";
import { formatBytes, formatDuration } from "../../components/format";
import { KIND_LABELS } from "../../components/labels";
import { jobTitle, userLabel } from "../../components/queue-summary";
import type { ActionRunner } from "../../components/use-action";
import { RequestActions } from "../request-actions";
import { MUTED } from "./styles";

function demoText(demo: AdminJob["demo"]): string {
  const name = demo.fileName ?? "demo.dem";
  return demo.sizeBytes === null ? name : `${name}, ${formatBytes(demo.sizeBytes)}`;
}

export interface RequestsProps {
  jobs: AdminJob[];
  actions: ActionRunner;
}

// The operator's inbox: nothing here is recorded until it is approved.
export function Requests({ jobs, actions }: RequestsProps) {
  return (
    <ul className="flex flex-col gap-3">
      {jobs.map((job) => (
        <li key={job.id}>
          <Panel tone="warning" className="grid gap-x-6 gap-y-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
            <div className="flex min-w-0 flex-col gap-1">
              <TextLink className="font-semibold wrap-anywhere" href={`/admin/jobs/${job.id}`}>
                {jobTitle(job)}
              </TextLink>
              <span className="wrap-anywhere">
                {userLabel(job.user)}
                {job.user.name !== null && job.user.email !== null && (
                  <span className="text-fg-2"> ({job.user.email})</span>
                )}
              </span>
              <span className={MUTED}>
                {KIND_LABELS[job.kind]}
                {job.estimatedSeconds !== null && ` · unos ${formatDuration(job.estimatedSeconds)} de máquina`} ·{" "}
                {demoText(job.demo)}
                {job.waitedSeconds !== null && ` · espera desde hace ${formatDuration(job.waitedSeconds)}`}
              </span>
            </div>
            <RequestActions job={job} actions={actions} size="sm" className="lg:justify-end" />
          </Panel>
        </li>
      ))}
    </ul>
  );
}
