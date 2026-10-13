"use client";

import { Panel } from "@/components/panel";
import { SectionHeading } from "@/components/section-heading";
import { TextLink } from "@/components/text-link";

import { ConfirmButton } from "../components/confirm-button";
import { ActionError, Empty, LoadError, Loading } from "../components/feedback";
import { formatDate, formatRelative } from "../components/format";
import { parseStudioDevices } from "../components/parse";
import { useAction } from "../components/use-action";
import { usePoll } from "../components/use-poll";

export function DevicesView() {
  const poll = usePoll({ url: "/api/studio/devices", intervalMs: 30000, parse: parseStudioDevices });
  const actions = useAction(poll.refresh);
  const devices = poll.data?.devices ?? null;
  const now = Date.now();

  return (
    <section className="mt-10">
      <SectionHeading title="Dispositivos" />
      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      <ActionError message={actions.error} />
      {devices === null && poll.error === null && <Loading>Cargando tus dispositivos.</Loading>}

      {devices !== null && devices.length === 0 && (
        <Empty>
          <p className="text-fg-1">No tienes ningún ClipHub Studio vinculado a esta cuenta.</p>
          <p>
            En Studio, elige «Nube ClipHub» y pulsa «Conecta tu cuenta». Te dará un código para{" "}
            <TextLink href="/link">vincular el dispositivo</TextLink>.
          </p>
        </Empty>
      )}

      <ul className="flex flex-col gap-3">
        {(devices ?? []).map((device) => (
          <li key={device.id}>
            <Panel className="flex-row flex-wrap items-center justify-between">
              <div className="flex min-w-0 flex-1 basis-48 flex-col gap-0.5 wrap-anywhere">
                <strong className="font-semibold">{device.name}</strong>
                <span className="text-body-sm text-fg-2">
                  Vinculado el {formatDate(device.createdAt)} ·{" "}
                  {device.lastSeenAt === null
                    ? "sin actividad todavía"
                    : `activo ${formatRelative(device.lastSeenAt, now)}`}
                </span>
              </div>
              <ConfirmButton
                label="Desvincular"
                confirmLabel="Sí, desvincular"
                size="sm"
                disabled={actions.busyKey !== null}
                busy={actions.busyKey === `revoke:${device.id}`}
                onConfirm={() =>
                  void actions.run(`revoke:${device.id}`, {
                    url: `/api/studio/devices/${encodeURIComponent(device.id)}`,
                    method: "DELETE",
                  })
                }
              />
            </Panel>
          </li>
        ))}
      </ul>
    </section>
  );
}
