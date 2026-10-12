import { Panel } from "@/components/panel";
import { cn } from "@/lib/utils";

import type { AdminStats } from "../../components/api-types";
import { Meter } from "../../components/feedback";
import { formatBytes, formatDuration, percentOf } from "../../components/format";
import { CELL_LABEL, MUTED, RULED, SPLIT_ROW } from "./styles";

const LOW_STORAGE_BYTES = 15 * 1024 ** 3;

export const FAILED_HISTORY = "/admin/history?status=failed";

export function isLowStorage(storage: AdminStats["storage"]): boolean {
  return storage.freeBytes < LOW_STORAGE_BYTES;
}

export function Stats({ stats }: { stats: AdminStats }) {
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

export function Storage({ storage }: { storage: AdminStats["storage"] }) {
  const low = isLowStorage(storage);
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
