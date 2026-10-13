"use client";

import { Panel } from "@/components/panel";
import { TextLink } from "@/components/text-link";

import { Empty, LoadError, Loading } from "../../components/feedback";
import { eventDetailText, eventLabel } from "../../components/labels";
import { parseAdminEvents, parseAdminWorkers } from "../../components/parse";
import { usePoll } from "../../components/use-poll";
import { Timeline, TimelineItem } from "../timeline";

export function ActivityView() {
  const poll = usePoll({ url: "/api/admin/events?limit=100", intervalMs: 10000, parse: parseAdminEvents });
  // Only used to name the worker of an event; the list works without it.
  const workersPoll = usePoll({ url: "/api/admin/workers", intervalMs: 60000, parse: parseAdminWorkers });
  const events = poll.data?.events ?? null;
  const workerNames = new Map((workersPoll.data?.workers ?? []).map((worker) => [worker.id, worker.name]));

  return (
    <>
      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      {events === null && poll.error === null && <Loading>Cargando la actividad.</Loading>}
      {events !== null && events.length === 0 && <Empty>Todavía no hay actividad registrada.</Empty>}
      {events !== null && events.length > 0 && (
        <Panel>
          <Timeline>
            {events.map((event) => {
              const detail = eventDetailText(event.type, event.detail);
              const workerName = event.workerId === null ? undefined : workerNames.get(event.workerId);
              return (
                <TimelineItem key={event.id} at={event.at} actor={event.actor}>
                  <strong className="font-semibold">{eventLabel(event.type)}</strong>
                  {workerName !== undefined && <span className="text-fg-2"> · {workerName}</span>}
                  {detail !== null && detail !== workerName && <span className="text-fg-2"> · {detail}</span>}
                  {event.requestId !== null && (
                    <>
                      <span className="text-fg-2"> · </span>
                      <TextLink href={`/admin/jobs/${encodeURIComponent(event.requestId)}`}>Ver trabajo</TextLink>
                    </>
                  )}
                  {event.subjectUserId !== null && (
                    <>
                      <span className="text-fg-2"> · </span>
                      <TextLink href={`/admin/history?userId=${encodeURIComponent(event.subjectUserId)}`}>
                        Trabajos del usuario
                      </TextLink>
                    </>
                  )}
                </TimelineItem>
              );
            })}
          </Timeline>
        </Panel>
      )}
    </>
  );
}
