"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Panel } from "@/components/panel";
import { TextLink } from "@/components/text-link";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";

import { JOB_KINDS, JOB_STATUSES } from "../../components/api-types";
import type { AdminJob, AdminUser } from "../../components/api-types";
import { ActionError, Empty, LoadError, Loading } from "../../components/feedback";
import { fetchJson } from "../../components/fetch-json";
import { formatDateTime, formatDuration } from "../../components/format";
import { KIND_LABELS, STATUS_LABELS } from "../../components/labels";
import { parseAdminJobs, parseAdminUsers } from "../../components/parse";
import { jobTitle, userLabel } from "../../components/queue-summary";
import { StatusPill } from "../../components/status-pill";
import { useAction } from "../../components/use-action";
import type { ActionRunner } from "../../components/use-action";
import { FailureSummary, RawCause } from "../failure";
import { hasJobActions, JobActions, retryBlocker } from "../job-actions";

const PAGE_SIZE = 50;
const FIELD = "grid w-full gap-2 sm:w-56";
const MUTED = "text-body-sm text-fg-2";

export interface HistoryFilters {
  status: string;
  kind: string;
  userId: string;
}

function historyUrl(filters: HistoryFilters, before: number | null): string {
  const params = new URLSearchParams();
  if (filters.status !== "") params.set("status", filters.status);
  if (filters.kind !== "") params.set("kind", filters.kind);
  if (filters.userId.trim() !== "") params.set("userId", filters.userId.trim());
  if (before !== null) params.set("before", String(before));
  params.set("limit", String(PAGE_SIZE));
  return `/api/admin/jobs?${params.toString()}`;
}

function userOption(user: AdminUser): string {
  if (user.name !== null && user.email !== null) return `${user.name} (${user.email})`;
  return user.name ?? user.email ?? `Usuario ${user.id.slice(0, 8)}`;
}

interface UserPickerProps {
  value: string;
  onChange: (userId: string) => void;
}

// The people of the portal by name, so nobody has to know or type a user id.
function UserPicker({ value, onChange }: UserPickerProps) {
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let current = true;
    void fetchJson("/api/admin/users", parseAdminUsers).then((result) => {
      if (!current) return;
      if (!result.ok) {
        setFailed(true);
        return;
      }
      const sorted = [...result.data.users].sort((a, b) => userOption(a).localeCompare(userOption(b), "es"));
      setUsers(sorted);
    });
    return () => {
      current = false;
    };
  }, []);

  // A link from another page can carry a user this list does not have (yet, or any more).
  const listed = users?.some((user) => user.id === value) ?? false;
  return (
    <div className={FIELD}>
      <Label htmlFor="history-user">Usuario</Label>
      <NativeSelect id="history-user" value={value} onChange={(event) => onChange(event.target.value)}>
        <NativeSelectOption value="">Todos</NativeSelectOption>
        {value !== "" && !listed && (
          <NativeSelectOption value={value}>
            {users === null && !failed ? "Cargando usuarios" : `Usuario ${value.slice(0, 8)}`}
          </NativeSelectOption>
        )}
        {(users ?? []).map((user) => (
          <NativeSelectOption key={user.id} value={user.id}>
            {userOption(user)}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      {failed && <span className={MUTED}>No se pudo cargar la lista de usuarios.</span>}
    </div>
  );
}

function HistoryRow({ job, actions }: { job: AdminJob; actions: ActionRunner }) {
  const ended = job.status === "failed" || job.status === "canceled";
  const legacy = job.legacyStatus || job.kind === "manual";
  const blocker = ended && !legacy ? retryBlocker(job) : null;
  return (
    <li>
      <Panel>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <StatusPill status={job.status} stage={job.stage} />
          <TextLink className="min-w-0 font-semibold wrap-anywhere" href={`/admin/jobs/${job.id}`}>
            {jobTitle(job)}
          </TextLink>
          <span className={MUTED}>
            {userLabel(job.user)} · {KIND_LABELS[job.kind]} · {formatDateTime(job.finishedAt ?? job.createdAt)}
            {job.machineSeconds > 0 && ` · ${formatDuration(job.machineSeconds)} de máquina`}
            {job.attempt > 0 && ` · intento ${job.attempt} de ${job.maxAttempts}`}
          </span>
        </div>

        {job.status === "canceled" && job.canceledBy !== null && (
          <p className={MUTED}>Cancelado por {job.canceledBy === "admin" ? "un operador" : "el usuario"}.</p>
        )}

        {job.failure !== null && (
          <>
            <FailureSummary failure={job.failure} />
            {job.failureDetail !== null && <RawCause detail={job.failureDetail} open />}
          </>
        )}

        {(hasJobActions(job) || blocker !== null) && (
          <div className="flex flex-wrap items-center gap-2">
            {hasJobActions(job) && <JobActions job={job} actions={actions} />}
            {blocker !== null && <span className={MUTED}>No se puede reintentar: {blocker}</span>}
          </div>
        )}
      </Panel>
    </li>
  );
}

export function HistoryView({ initial }: { initial: HistoryFilters }) {
  const [filters, setFilters] = useState<HistoryFilters>(initial);
  const [jobs, setJobs] = useState<AdminJob[]>([]);
  const [nextBefore, setNextBefore] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<{ code: string; detail: string | null } | null>(null);
  const [loadedAt, setLoadedAt] = useState<number | null>(null);
  const sequence = useRef(0);

  const load = useCallback(
    async (before: number | null) => {
      const ticket = ++sequence.current;
      setLoading(true);
      const result = await fetchJson(historyUrl(filters, before), parseAdminJobs);
      if (ticket !== sequence.current) return;
      setLoading(false);
      if (!result.ok) {
        setError({ code: result.code, detail: result.detail });
        return;
      }
      setError(null);
      setLoadedAt(Date.now());
      setJobs((current) => (before === null ? result.data.jobs : [...current, ...result.data.jobs]));
      setNextBefore(result.data.nextBefore);
    },
    [filters],
  );

  useEffect(() => {
    void load(null);
  }, [load]);

  const reload = useCallback(() => void load(null), [load]);
  const actions = useAction(reload);

  return (
    <>
      <form className="mb-5 flex flex-wrap items-end gap-3" onSubmit={(event) => event.preventDefault()}>
        <div className={FIELD}>
          <Label htmlFor="history-status">Estado</Label>
          <NativeSelect
            id="history-status"
            value={filters.status}
            onChange={(event) => setFilters({ ...filters, status: event.target.value })}
          >
            <NativeSelectOption value="">Todos</NativeSelectOption>
            {JOB_STATUSES.map((status) => (
              <NativeSelectOption key={status} value={status}>
                {STATUS_LABELS[status]}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        <div className={FIELD}>
          <Label htmlFor="history-kind">Tipo</Label>
          <NativeSelect
            id="history-kind"
            value={filters.kind}
            onChange={(event) => setFilters({ ...filters, kind: event.target.value })}
          >
            <NativeSelectOption value="">Todos</NativeSelectOption>
            {JOB_KINDS.map((kind) => (
              <NativeSelectOption key={kind} value={kind}>
                {KIND_LABELS[kind]}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        <UserPicker value={filters.userId} onChange={(userId) => setFilters({ ...filters, userId })} />
        <Button type="button" variant="outline" onClick={reload} disabled={loading}>
          Actualizar
        </Button>
      </form>

      {error !== null && <LoadError error={error} onRetry={reload} staleSince={jobs.length > 0 ? loadedAt : null} />}
      <ActionError message={actions.error} />

      {jobs.length === 0 && !loading && error === null && <Empty>Ningún trabajo coincide con estos filtros.</Empty>}
      {jobs.length === 0 && loading && <Loading>Cargando el historial.</Loading>}

      <ul className="flex flex-col gap-3">
        {jobs.map((job) => (
          <HistoryRow key={job.id} job={job} actions={actions} />
        ))}
      </ul>

      {nextBefore !== null && (
        <Button type="button" variant="outline" className="mt-4" disabled={loading} onClick={() => void load(nextBefore)}>
          {loading ? "Cargando" : "Cargar más"}
        </Button>
      )}
    </>
  );
}
