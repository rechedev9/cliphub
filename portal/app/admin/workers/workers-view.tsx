"use client";

import { useState } from "react";

import { Panel } from "@/components/panel";
import { StatusDot, TONE_TEXT } from "@/components/status-dot";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

import type { AdminWorker } from "../../components/api-types";
import { ConfirmButton } from "../../components/confirm-button";
import { ActionError, Empty, LoadError, Loading } from "../../components/feedback";
import { formatDateTime, formatRelative } from "../../components/format";
import { reasonLabel, splitPauseReason } from "../../components/labels";
import { parseAdminWorkerCreated, parseAdminWorkers } from "../../components/parse";
import { workerStatus } from "../../components/queue-summary";
import { useAction } from "../../components/use-action";
import type { ActionRunner } from "../../components/use-action";
import { usePoll } from "../../components/use-poll";
import { PauseControl, WORKER_CARD_TONE } from "../worker-card";

const NAME_MAX = 60;
const MUTED = "text-body-sm text-fg-2";
const CODE = "rounded-sm border border-border-subtle bg-surface-3 px-1.5 font-mono text-[0.9em] wrap-anywhere";

function TokenReveal({ token, onDismiss }: { token: string; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false);
  const origin = typeof window === "undefined" ? "" : window.location.origin;

  async function copy() {
    try {
      await navigator.clipboard.writeText(token);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <Card tone="warning" role="status" className="mb-6">
      <CardContent className="flex flex-col items-start gap-3">
        <h3 className="text-body-lg font-semibold">Token del nuevo worker</h3>
        <p className="text-warning">
          Cópialo ahora. No se puede volver a mostrar: si lo pierdes, revoca este worker y crea otro.
        </p>
        <div className="flex w-full flex-wrap items-center gap-2">
          <label className="sr-only" htmlFor="worker-token">
            Token
          </label>
          <Input
            id="worker-token"
            className="flex-1 basis-56 font-mono"
            type="text"
            readOnly
            value={token}
            onFocus={(event) => event.target.select()}
          />
          <Button type="button" onClick={() => void copy()}>
            {copied ? "Copiado" : "Copiar"}
          </Button>
        </div>
        <p>En el servidor, define estas dos variables de entorno del usuario de Windows y reinicia Studio:</p>
        <ul className="flex list-disc flex-col gap-1.5 pl-4.5 text-fg-2">
          <li>
            <code className={CODE}>ZV_BRIDGE_URL</code> con el valor <code className={CODE}>{origin}</code>
          </li>
          <li>
            <code className={CODE}>ZV_BRIDGE_TOKEN</code> con el token de arriba
          </li>
        </ul>
        <Button type="button" variant="outline" size="sm" onClick={onDismiss}>
          Ya lo he guardado
        </Button>
      </CardContent>
    </Card>
  );
}

interface WorkerRowProps {
  worker: AdminWorker;
  now: number;
  actions: ActionRunner;
}

function WorkerRow({ worker, now, actions }: WorkerRowProps) {
  const [renaming, setRenaming] = useState(false);
  const [name, setName] = useState(worker.name);
  const status = workerStatus(worker, null);
  const busy = actions.busyKey !== null;
  const base = `/api/admin/workers/${worker.id}`;

  return (
    <li>
      <Panel tone={WORKER_CARD_TONE[status.tone]}>
        <div className="flex items-center gap-3">
          <StatusDot tone={status.tone} className="size-3" />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            {renaming ? (
              <form
                className="flex flex-wrap items-center gap-2"
                onSubmit={(event) => {
                  event.preventDefault();
                  if (name.trim() === "") return;
                  setRenaming(false);
                  void actions.run(`rename:${worker.id}`, { url: `${base}/rename`, body: { name: name.trim() } });
                }}
              >
                <label className="sr-only" htmlFor={`rename-${worker.id}`}>
                  Nombre del worker
                </label>
                <Input
                  id={`rename-${worker.id}`}
                  className="h-8 flex-1 basis-48 px-2.5"
                  type="text"
                  value={name}
                  maxLength={NAME_MAX}
                  autoFocus
                  onChange={(event) => setName(event.target.value)}
                />
                <Button type="submit" size="xs" disabled={busy || name.trim() === ""}>
                  Guardar
                </Button>
                <Button type="button" variant="outline" size="xs" onClick={() => setRenaming(false)}>
                  No
                </Button>
              </form>
            ) : (
              <h3 className="text-body-lg leading-tight font-semibold wrap-anywhere">{worker.name}</h3>
            )}
            <span className={MUTED}>
              Creado el {formatDateTime(worker.createdAt)} ·{" "}
              {worker.lastSeenAt === null
                ? "nunca se ha conectado"
                : `última señal ${formatRelative(worker.lastSeenAt, now)}`}
              {worker.health !== null && worker.health.hostname !== "" && ` · ${worker.health.hostname}`}
            </span>
          </div>
          <span className={cn("font-mono text-label whitespace-nowrap uppercase", TONE_TEXT[status.tone])}>
            {status.label}
          </span>
        </div>

        {worker.paused && !worker.revoked && (
          <p className={MUTED}>
            {worker.pausedBy === "auto" ? "Pausado automáticamente: " : "Pausado por un operador: "}
            {splitPauseReason(worker.pauseReason).label}
          </p>
        )}
        {worker.blocked !== null && worker.online && !worker.revoked && (
          <p className={MUTED}>
            Bloqueado: {reasonLabel(worker.blocked.code)}
            {worker.blocked.detail !== "" && ` (${worker.blocked.detail})`}
          </p>
        )}
        {worker.revoked && <p className={MUTED}>Su token ya no sirve. No se puede recuperar.</p>}

        {!worker.revoked && !renaming && (
          <div className="flex flex-wrap items-center gap-2">
            <PauseControl worker={worker} actions={actions} />
            <Button
              type="button"
              variant="outline"
              size="xs"
              disabled={busy}
              onClick={() => {
                setName(worker.name);
                setRenaming(true);
              }}
            >
              Renombrar
            </Button>
            <ConfirmButton
              label="Revocar"
              confirmLabel="Sí, revocar el token"
              disabled={busy}
              busy={actions.busyKey === `revoke:${worker.id}`}
              onConfirm={() => void actions.run(`revoke:${worker.id}`, { url: `${base}/revoke` })}
            />
          </div>
        )}
      </Panel>
    </li>
  );
}

export function WorkersView() {
  const poll = usePoll({ url: "/api/admin/workers", intervalMs: 5000, parse: parseAdminWorkers });
  const actions = useAction(poll.refresh);
  const [newName, setNewName] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);
  const workers = poll.data?.workers ?? null;
  const now = Date.now();

  async function create() {
    setCreateError(null);
    const result = await actions.run("create", { url: "/api/admin/workers", body: { name: newName.trim() } });
    if (!result.ok) return;
    try {
      setToken(parseAdminWorkerCreated(result.data).token);
      setNewName("");
    } catch {
      setCreateError("El worker se ha creado, pero el portal no devolvió el token. Revócalo y crea otro.");
    }
  }

  return (
    <>
      <form
        className="mb-6"
        onSubmit={(event) => {
          event.preventDefault();
          void create();
        }}
      >
        <Card>
          <CardContent className="flex flex-col gap-3">
            <h2 className="text-body-lg font-semibold">Nuevo worker</h2>
            <p className="text-fg-2">
              Un worker es un PC con ClipHub Studio que graba los trabajos de la cola. Cada uno tiene su propio token.
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <label className="sr-only" htmlFor="worker-name">
                Nombre del worker
              </label>
              <Input
                id="worker-name"
                className="flex-1 basis-56"
                type="text"
                value={newName}
                maxLength={NAME_MAX}
                placeholder="Nombre, por ejemplo capture-01"
                onChange={(event) => setNewName(event.target.value)}
              />
              <Button type="submit" disabled={actions.busyKey !== null || newName.trim() === ""}>
                Crear worker
              </Button>
            </div>
            {createError !== null && (
              <p className="text-destructive" role="alert">
                {createError}
              </p>
            )}
          </CardContent>
        </Card>
      </form>

      {token !== null && <TokenReveal token={token} onDismiss={() => setToken(null)} />}

      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      <ActionError message={actions.error} />
      {workers === null && poll.error === null && <Loading>Cargando workers.</Loading>}
      {workers !== null && workers.length === 0 && (
        <Empty>Todavía no hay ningún worker. Crea el primero con el formulario de arriba.</Empty>
      )}

      <ul className="flex flex-col gap-3">
        {(workers ?? []).map((worker) => (
          <WorkerRow key={worker.id} worker={worker} now={now} actions={actions} />
        ))}
      </ul>
    </>
  );
}
