import { Panel } from "@/components/panel";
import { TextLink } from "@/components/text-link";
import { Badge } from "@/components/ui/badge";

import type { AdminJob } from "../../components/api-types";
import { formatRelative } from "../../components/format";
import { FAILURE_LABELS } from "../../components/labels";
import { causeHeadline, jobTitle, userLabel } from "../../components/queue-summary";
import { MUTED } from "./styles";

export function RecentFailures({ jobs, now }: { jobs: AdminJob[]; now: number }) {
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
