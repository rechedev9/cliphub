"use client";

import { useEffect, useState } from "react";

import { Facts, Fact } from "@/components/facts";
import { Panel } from "@/components/panel";
import { TextLink } from "@/components/text-link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Textarea } from "@/components/ui/textarea";

import { CLOUD_ACCESS } from "../../components/api-types";
import type { AdminUser, CloudAccess } from "../../components/api-types";
import { ActionError, Empty, LoadError, Loading } from "../../components/feedback";
import { formatDuration, formatRelative } from "../../components/format";
import { ACCESS_LABELS } from "../../components/labels";
import { parseAdminUsers } from "../../components/parse";
import { useAction } from "../../components/use-action";
import type { ActionRunner } from "../../components/use-action";
import { usePoll } from "../../components/use-poll";

const ACCESS_VARIANT = {
  pending: "warning",
  allowed: "success",
  blocked: "danger",
} as const satisfies Record<CloudAccess, string>;

const MUTED = "text-body-sm text-fg-2";

// An empty field clears the override; anything else must be a whole number of at least `min`.
function parseOverride(text: string, min: number): number | null | "invalid" {
  const trimmed = text.trim();
  if (trimmed === "") return null;
  if (!/^\d+$/.test(trimmed)) return "invalid";
  const value = Number(trimmed);
  return value >= min ? value : "invalid";
}

interface EditorProps {
  user: AdminUser;
  actions: ActionRunner;
  onClose: () => void;
}

function Editor({ user, actions, onClose }: EditorProps) {
  const [maxActive, setMaxActive] = useState(user.overrides.maxActive === null ? "" : String(user.overrides.maxActive));
  const [dailyMinutes, setDailyMinutes] = useState(
    user.overrides.dailySeconds === null ? "" : String(Math.round(user.overrides.dailySeconds / 60)),
  );
  const [note, setNote] = useState(user.note ?? "");
  const [invalid, setInvalid] = useState(false);

  async function save() {
    const active = parseOverride(maxActive, 1);
    const minutes = parseOverride(dailyMinutes, 1);
    if (active === "invalid" || minutes === "invalid") {
      setInvalid(true);
      return;
    }
    setInvalid(false);
    const result = await actions.run(`save:${user.id}`, {
      url: `/api/admin/users/${user.id}`,
      body: {
        maxActive: active,
        dailySeconds: minutes === null ? null : minutes * 60,
        note: note.trim() === "" ? null : note.trim(),
      },
    });
    if (result.ok) onClose();
  }

  return (
    <form
      className="flex flex-col gap-4 border-t border-border-subtle pt-4"
      onSubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="grid gap-2">
          <Label htmlFor={`max-active-${user.id}`}>Trabajos activos a la vez</Label>
          <Input
            id={`max-active-${user.id}`}
            type="text"
            inputMode="numeric"
            value={maxActive}
            placeholder={user.overrides.maxActive === null ? `Por defecto (${user.limits.maxActive})` : "Por defecto"}
            onChange={(event) => setMaxActive(event.target.value)}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor={`daily-${user.id}`}>Minutos de máquina cada 24 h</Label>
          <Input
            id={`daily-${user.id}`}
            type="text"
            inputMode="numeric"
            value={dailyMinutes}
            placeholder={
              user.overrides.dailySeconds === null
                ? `Por defecto (${Math.round(user.limits.dailySeconds / 60)})`
                : "Por defecto"
            }
            onChange={(event) => setDailyMinutes(event.target.value)}
          />
        </div>
      </div>
      <div className="grid gap-2">
        <Label htmlFor={`note-${user.id}`}>Nota interna</Label>
        <Textarea
          id={`note-${user.id}`}
          value={note}
          maxLength={500}
          onChange={(event) => setNote(event.target.value)}
        />
      </div>
      {invalid && (
        <p className="text-destructive" role="alert">
          Los límites tienen que ser números enteros mayores que cero. Deja el campo vacío para usar el valor por defecto.
        </p>
      )}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <Button type="submit" size="sm" disabled={actions.busyKey !== null}>
          Guardar
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onClose}>
          Cerrar
        </Button>
        <TextLink className="text-body-sm" href={`/admin/history?userId=${encodeURIComponent(user.id)}`}>
          Ver sus trabajos
        </TextLink>
      </div>
    </form>
  );
}

interface UserRowProps {
  user: AdminUser;
  now: number;
  actions: ActionRunner;
}

function UserRow({ user, now, actions }: UserRowProps) {
  const [editing, setEditing] = useState(false);
  const busy = actions.busyKey !== null;
  const setAccess = (access: CloudAccess) =>
    void actions.run(`access:${user.id}`, { url: `/api/admin/users/${user.id}`, body: { access } });
  const overridden = user.overrides.maxActive !== null || user.overrides.dailySeconds !== null;

  return (
    <li>
      {/* Someone waiting for access is the one row here that asks for a decision. */}
      <Panel tone={user.access === "pending" ? "warning" : "neutral"}>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <div className="flex min-w-0 flex-1 basis-48 flex-col gap-0.5 wrap-anywhere">
            <strong className="font-semibold">{user.name ?? user.email ?? user.id}</strong>
            {user.name !== null && user.email !== null && <span className={MUTED}>{user.email}</span>}
          </div>
          <Badge variant={ACCESS_VARIANT[user.access]} className="font-mono uppercase">
            {ACCESS_LABELS[user.access]}
          </Badge>
          {user.isAdmin && (
            <Badge variant="outline" className="font-mono uppercase">
              Admin
            </Badge>
          )}
        </div>

        <Facts>
          <Fact label="Uso en 24 h">
            {formatDuration(user.usage.secondsLast24h)} de {formatDuration(user.limits.dailySeconds)}
          </Fact>
          <Fact label="Activos ahora">
            {user.usage.active} de {user.limits.maxActive}
          </Fact>
          <Fact label="Trabajos en 7 días">{user.usage.jobs7d}</Fact>
          <Fact label="Fallos en 7 días" className={user.usage.failed7d > 0 ? "text-warning" : undefined}>
            {user.usage.failed7d}
          </Fact>
          <Fact label="Dispositivos">{user.devices}</Fact>
          <Fact label="Último trabajo">
            {user.lastJobAt === null ? "Ninguno" : formatRelative(user.lastJobAt, now)}
          </Fact>
        </Facts>

        {overridden && <p className={MUTED}>Tiene límites propios, distintos de los generales.</p>}
        {user.note !== null && !editing && <p className={MUTED}>Nota: {user.note}</p>}

        {editing ? (
          <Editor user={user} actions={actions} onClose={() => setEditing(false)} />
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            {user.isAdmin ? (
              <span className={MUTED}>Los administradores siempre tienen acceso.</span>
            ) : (
              <>
                {user.access !== "allowed" && (
                  <Button type="button" size="xs" disabled={busy} onClick={() => setAccess("allowed")}>
                    Permitir
                  </Button>
                )}
                {user.access !== "blocked" && (
                  <Button
                    type="button"
                    variant="outline-destructive"
                    size="xs"
                    disabled={busy}
                    onClick={() => setAccess("blocked")}
                  >
                    Bloquear
                  </Button>
                )}
              </>
            )}
            <Button type="button" variant="outline" size="xs" onClick={() => setEditing(true)}>
              Límites y nota
            </Button>
          </div>
        )}
      </Panel>
    </li>
  );
}

export function UsersView({ initialAccess }: { initialAccess: string }) {
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [access, setAccess] = useState(initialAccess);

  // The search waits for a pause in typing so each key press is not a request.
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(query.trim()), 300);
    return () => clearTimeout(timer);
  }, [query]);

  const params = new URLSearchParams();
  if (debounced !== "") params.set("query", debounced);
  if (access !== "") params.set("access", access);
  const suffix = params.toString();

  const poll = usePoll({
    url: suffix === "" ? "/api/admin/users" : `/api/admin/users?${suffix}`,
    intervalMs: 15000,
    parse: parseAdminUsers,
  });
  const actions = useAction(poll.refresh);
  const users = poll.data?.users ?? null;
  const now = Date.now();

  return (
    <>
      <form className="mb-5 flex flex-wrap items-end gap-3" onSubmit={(event) => event.preventDefault()}>
        <div className="grid min-w-0 flex-1 basis-64 gap-2">
          <Label htmlFor="users-query">Buscar por nombre o correo</Label>
          <Input id="users-query" type="search" value={query} onChange={(event) => setQuery(event.target.value)} />
        </div>
        <div className="grid w-full gap-2 sm:w-56">
          <Label htmlFor="users-access">Acceso</Label>
          <NativeSelect id="users-access" value={access} onChange={(event) => setAccess(event.target.value)}>
            <NativeSelectOption value="">Todos</NativeSelectOption>
            {CLOUD_ACCESS.map((value) => (
              <NativeSelectOption key={value} value={value}>
                {ACCESS_LABELS[value]}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      </form>

      {poll.error !== null && <LoadError error={poll.error} onRetry={poll.refresh} staleSince={poll.loadedAt} />}
      <ActionError message={actions.error} />
      {users === null && poll.error === null && <Loading>Cargando usuarios.</Loading>}
      {users !== null && users.length === 0 && <Empty>Ningún usuario coincide con la búsqueda.</Empty>}

      <ul className="flex flex-col gap-3">
        {(users ?? []).map((user) => (
          <UserRow key={user.id} user={user} now={now} actions={actions} />
        ))}
      </ul>
    </>
  );
}
