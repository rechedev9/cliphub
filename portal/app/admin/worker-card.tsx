"use client";

import { useState } from "react";

import { Facts, Fact } from "@/components/facts";
import { Notice, NoticeDetail } from "@/components/notice";
import { Panel } from "@/components/panel";
import { StatusDot, TONE_TEXT } from "@/components/status-dot";
import type { StatusTone } from "@/components/status-dot";
import { TextLink } from "@/components/text-link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

import type { AdminJob, AdminWorker } from "../components/api-types";
import { Meter } from "../components/feedback";
import { formatBytes, formatClock, formatDuration, formatRelative, percentOf } from "../components/format";
import { reasonLabel, splitPauseReason, STAGE_LABELS } from "../components/labels";
import { forgotItsJob, jobTitle, leaseOutlook, userLabel, workerStatus } from "../components/queue-summary";
import type { ActionRunner } from "../components/use-action";

const LOW_DISK_PERCENT = 15;

// The card edge repeats the state, so a row of workers reads at a glance.
export const WORKER_CARD_TONE = {
  ok: "neutral",
  live: "stream",
  warn: "warning",
  bad: "danger",
  off: "neutral",
} as const satisfies Record<StatusTone, string>;

const CODE = "rounded-sm border border-border-subtle bg-surface-3 px-1 font-mono text-[0.9em]";
const SPLIT_ROW = "flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5";

export interface PauseControlProps {
  worker: AdminWorker;
  actions: ActionRunner;
}

export function PauseControl({ worker, actions }: PauseControlProps) {
  const [asking, setAsking] = useState(false);
  const [reason, setReason] = useState("");
  const busy = actions.busyKey !== null;
  const base = `/api/admin/workers/${worker.id}`;

  if (worker.revoked) return null;

  if (worker.paused) {
    return (
      <Button
        type="button"
        size="xs"
        disabled={busy}
        onClick={() => void actions.run(`resume:${worker.id}`, { url: `${base}/resume` })}
      >
        Reanudar
      </Button>
    );
  }

  if (!asking) {
    return (
      <Button type="button" variant="outline" size="xs" disabled={busy} onClick={() => setAsking(true)}>
        Pausar
      </Button>
    );
  }

  return (
    <form
      className="flex w-full max-w-xl flex-wrap items-center gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        setAsking(false);
        void actions.run(`pause:${worker.id}`, { url: `${base}/pause`, body: { reason: reason.trim() } });
        setReason("");
      }}
    >
      <label className="sr-only" htmlFor={`pause-reason-${worker.id}`}>
        Motivo de la pausa
      </label>
      <Input
        id={`pause-reason-${worker.id}`}
        className="h-8 flex-1 basis-48 px-2.5"
        type="text"
        value={reason}
        maxLength={300}
        placeholder="Motivo de la pausa"
        autoFocus
        onChange={(event) => setReason(event.target.value)}
      />
      <Button type="submit" size="xs" disabled={busy}>
        Pausar
      </Button>
      <Button type="button" variant="outline" size="xs" onClick={() => setAsking(false)}>
        No
      </Button>
    </form>
  );
}

function PausedBanner({ worker }: { worker: AdminWorker }) {
  const reason = splitPauseReason(worker.pauseReason);
  const auto = worker.pausedBy === "auto";
  const incompatible = worker.pauseReason?.startsWith("capture_incompatible") === true;
  return (
    <Notice tone="danger" role="status">
      <div className="min-w-0">
        <strong className="font-semibold">
          {auto ? "Pausado automáticamente: " : "Pausado por un operador: "}
          {reason.label}
        </strong>
        {reason.job !== null && (
          <NoticeDetail>
            Lo causó el trabajo{" "}
            <TextLink href={`/admin/jobs/${encodeURIComponent(reason.job.id)}`}>
              {reason.job.title === "" ? "sin título" : reason.job.title}
            </TextLink>
            . Si vuelve a pasar con el mismo, el problema puede ser su demo y no la máquina.
          </NoticeDetail>
        )}
        {reason.detail !== null && <NoticeDetail className="font-mono">{reason.detail}</NoticeDetail>}
        {incompatible && (
          <NoticeDetail>
            Reinicia Studio en el servidor (descarga el último HLAE al arrancar) y pulsa Reanudar.
          </NoticeDetail>
        )}
        {!incompatible && auto && (
          <NoticeDetail>No toma trabajos hasta que pulses Reanudar. La cola se mantiene.</NoticeDetail>
        )}
      </div>
    </Notice>
  );
}

interface OfflineBannerProps {
  worker: AdminWorker;
  currentJob: AdminJob | null;
  now: number;
}

function OfflineBanner({ worker, currentJob, now }: OfflineBannerProps) {
  if (worker.lastSeenAt === null) {
    return (
      <Notice tone="warning" role="status">
        <div className="min-w-0">
          <strong className="font-semibold">Todavía no se ha conectado nunca.</strong>
          <NoticeDetail>
            Define <code className={CODE}>ZV_BRIDGE_URL</code> y <code className={CODE}>ZV_BRIDGE_TOKEN</code> en el
            servidor y reinicia Studio. Aparece aquí en unos 15 segundos.
          </NoticeDetail>
        </div>
      </Notice>
    );
  }
  return (
    <Notice tone="danger" role="status">
      <div className="min-w-0">
        <strong className="font-semibold">
          Sin señal desde las {formatClock(worker.lastSeenAt)} ({formatRelative(worker.lastSeenAt, now)})
        </strong>
        <NoticeDetail>
          {currentJob === null
            ? "No toma trabajos hasta que vuelva. La cola se mantiene."
            : `Tenía un trabajo a medias. ${leaseOutlook(currentJob, now)}`}
        </NoticeDetail>
        <NoticeDetail>
          Comprueba que el servidor está encendido, con la sesión de Windows abierta y Studio en marcha.
        </NoticeDetail>
      </div>
    </Notice>
  );
}

interface CurrentJobProps {
  job: AdminJob;
  now: number;
  // True when the worker is silent, so these are the last values it sent.
  stale: boolean;
}

function CurrentJob({ job, now, stale }: CurrentJobProps) {
  const percent = job.progressPercent ?? 0;
  const elapsed = job.startedAt === null ? null : (now - job.startedAt) / 1000;
  const expectedEnd =
    job.startedAt === null || job.estimatedSeconds === null ? null : job.startedAt + job.estimatedSeconds * 1000;
  return (
    <div className="flex flex-col gap-2 rounded-md border border-border-subtle bg-surface-3 p-3">
      <div className={SPLIT_ROW}>
        <TextLink className="font-semibold wrap-anywhere" href={`/admin/jobs/${job.id}`}>
          {jobTitle(job)}
        </TextLink>
        <span className="text-body-sm text-fg-2">{userLabel(job.user)}</span>
      </div>
      <div className={SPLIT_ROW}>
        <span>
          {job.stage === null ? "Preparando" : STAGE_LABELS[job.stage]}
          {job.progressDetail !== null && (
            <span className="font-mono text-body-sm text-fg-2"> · {job.progressDetail}</span>
          )}
        </span>
        <span className="font-mono tabular-nums">{job.progressPercent === null ? "" : `${percent} %`}</span>
      </div>
      <Meter percent={percent} tone={stale ? "off" : "live"} label="Progreso del trabajo actual" />
      <div className={cn(SPLIT_ROW, "text-body-sm text-fg-2")}>
        {elapsed !== null && <span>Lleva {formatDuration(elapsed)}</span>}
        {stale && <span>Último dato recibido</span>}
        {!stale && expectedEnd !== null && expectedEnd >= now && <span>Fin previsto {formatClock(expectedEnd)}</span>}
        {!stale && expectedEnd !== null && expectedEnd < now && (
          <span className="text-warning">
            Va {formatDuration((now - expectedEnd) / 1000)} por encima de lo estimado
          </span>
        )}
        <span>
          Intento {job.attempt} de {job.maxAttempts}
        </span>
      </div>
    </div>
  );
}

function version(value: string): string {
  return value === "" ? "desconocida" : value;
}

export interface WorkerCardProps {
  worker: AdminWorker;
  currentJob: AdminJob | null;
  now: number;
  actions: ActionRunner;
}

export function WorkerCard({ worker, currentJob, now, actions }: WorkerCardProps) {
  const status = workerStatus(worker, currentJob);
  const health = worker.health;
  const offline = !worker.online && !worker.revoked;
  const forgotten = forgotItsJob(worker, currentJob, now);
  const freePercent = health === null ? null : percentOf(health.diskFreeBytes, health.diskTotalBytes);
  const lowDisk = freePercent !== null && freePercent < LOW_DISK_PERCENT;

  return (
    <article>
      <Panel tone={WORKER_CARD_TONE[status.tone]} className="gap-3.5">
        <div className="flex items-center gap-3">
          <StatusDot tone={status.tone} className="size-3" />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <h3 className="text-body-lg leading-tight font-semibold wrap-anywhere">{worker.name}</h3>
            <span className="text-body-sm text-fg-2">
              {worker.lastSeenAt === null
                ? "Nunca se ha conectado"
                : `Última señal ${formatRelative(worker.lastSeenAt, now)}`}
            </span>
          </div>
          <span className={cn("font-mono text-label whitespace-nowrap uppercase", TONE_TEXT[status.tone])}>
            {status.label}
          </span>
        </div>

        {offline && <OfflineBanner worker={worker} currentJob={currentJob} now={now} />}
        {worker.paused && <PausedBanner worker={worker} />}

        {worker.blocked !== null && !offline && (
          <Notice tone="warning" role="status">
            <div className="min-w-0">
              <strong className="font-semibold">Bloqueado: {reasonLabel(worker.blocked.code)}</strong>
              {worker.blocked.detail !== "" && (
                <NoticeDetail className="font-mono">{worker.blocked.detail}</NoticeDetail>
              )}
              <NoticeDetail>No toma trabajos hasta que se resuelva. La cola se mantiene.</NoticeDetail>
            </div>
          </Notice>
        )}

        {currentJob !== null && <CurrentJob job={currentJob} now={now} stale={offline || forgotten} />}
        {forgotten && currentJob !== null && (
          <p className="text-body-sm text-warning">
            Dice estar libre, pero el portal le tiene asignado este trabajo. {leaseOutlook(currentJob, now)}
          </p>
        )}
        {currentJob === null && status.tone === "ok" && (
          <p className="text-body-sm text-fg-2">Esperando el siguiente trabajo.</p>
        )}

        {health === null && worker.lastSeenAt !== null && (
          <p className="text-body-sm text-fg-2">Todavía no ha enviado datos de la máquina.</p>
        )}
        {health !== null && (
          // Last known values of a silent worker: readable, but clearly not live.
          <div className={cn("flex flex-col gap-3.5", offline && "opacity-65 saturate-[0.25]")}>
            {offline && <p className="text-body-sm text-fg-2">Último estado conocido de la máquina:</p>}
            <div className="flex flex-col gap-1.5">
              <div className={SPLIT_ROW}>
                <span>Disco</span>
                <span className={cn("font-mono text-body-sm tabular-nums", lowDisk && "text-destructive")}>
                  {formatBytes(health.diskFreeBytes)} libres de {formatBytes(health.diskTotalBytes)}
                </span>
              </div>
              <Meter
                percent={100 - (freePercent ?? 0)}
                tone={lowDisk ? "bad" : "primary"}
                label="Espacio de disco usado en el worker"
              />
            </div>
            <Facts>
              <Fact label="Steam" className={health.steamRunning ? "text-success" : "text-destructive"}>
                {health.steamRunning ? "Abierto" : "Cerrado"}
              </Fact>
              <Fact label="CS2" className="font-mono">
                {version(health.cs2PatchVersion)}
              </Fact>
              <Fact label="HLAE" className="font-mono">
                {version(health.hlaeVersion)}
              </Fact>
              <Fact label="Studio" className="font-mono">
                {version(health.studioVersion)}
              </Fact>
              <Fact label="Versión del plan" className="font-mono">
                {version(health.planSchema)}
              </Fact>
              <Fact label="Equipo" className="font-mono">
                {health.hostname === "" ? "desconocido" : health.hostname}
              </Fact>
              <Fact label="Encendido">{formatDuration(health.uptimeSeconds)}</Fact>
            </Facts>
            {!health.recordEnabled && (
              <p className="text-destructive">La captura está desactivada en este Studio.</p>
            )}
          </div>
        )}

        <div className="flex flex-wrap items-center gap-2">
          <PauseControl worker={worker} actions={actions} />
        </div>
      </Panel>
    </article>
  );
}
