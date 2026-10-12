"use client";

import { Notice } from "@/components/notice";
import { SectionHeading } from "@/components/section-heading";
import { TextLink } from "@/components/text-link";
import { Button } from "@/components/ui/button";

import type { AdminJob, AdminOverview } from "../../components/api-types";
import { ActionError, Empty, LoadError, Loading } from "../../components/feedback";
import { parseAdminJobs, parseOverview } from "../../components/parse";
import { queueSummary, recentFailures } from "../../components/queue-summary";
import { useAction } from "../../components/use-action";
import { usePoll } from "../../components/use-poll";
import { WorkerCard } from "../worker-card";
import { ActiveJobs } from "./active-jobs";
import { FAILED_HISTORY, isLowStorage, Stats, Storage } from "./day-stats";
import { QueueTable } from "./queue-table";
import { RecentFailures } from "./recent-failures";
import { Requests } from "./requests";
import { MUTED } from "./styles";
import { Summary } from "./summary";

const MAX_RECENT_FAILURES = 3;

function findJob(overview: AdminOverview, id: string | null): AdminJob | null {
  if (id === null) return null;
  return overview.running.find((job) => job.id === id) ?? overview.uploading.find((job) => job.id === id) ?? null;
}

export function OverviewView() {
  const poll = usePoll({ url: "/api/admin/overview", intervalMs: 3000, parse: parseOverview });
  // A failed load here only hides the recent failures; the counter still comes from the overview.
  const failedPoll = usePoll({ url: "/api/admin/jobs?status=failed&limit=5", intervalMs: 15000, parse: parseAdminJobs });
  const actions = useAction(poll.refresh);
  const overview = poll.data;
  const failures = overview === null ? [] : recentFailures(failedPoll.data?.jobs ?? [], overview.now);
  const lowStorage = overview !== null && isLowStorage(overview.stats.storage);
  // With approval switched off, requests left from before still need an answer.
  const showRequests = overview !== null && (overview.approval === "required" || overview.requests.length > 0);

  return (
    <>
      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      <ActionError message={actions.error} />
      {overview === null && poll.error === null && <Loading>Cargando el estado de la nube.</Loading>}

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

          {showRequests && (
            <section id="solicitudes" className="mt-10 scroll-mt-6">
              <SectionHeading title="Solicitudes" count={overview.requests.length} />
              {overview.requests.length === 0 ? (
                <Empty>Ninguna solicitud pendiente. Las nuevas aparecen aquí y no se graban hasta que las apruebes.</Empty>
              ) : (
                <Requests jobs={overview.requests} actions={actions} />
              )}
            </section>
          )}
          {overview.approval === "off" && (
            <p className={`${MUTED} mt-4`}>
              La aprobación está desactivada (CLOUD_APPROVAL=off): los trabajos entran en la cola sin pasar por aquí.
            </p>
          )}

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
