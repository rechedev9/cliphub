'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { LoadingText } from '@/components/ui/loading-text';
import { FormError } from '@/components/clipmind/auth-forms';
import { LoadErrorState } from '@/components/records/load-error-state';
import {
  accountLabel,
  cm,
  isDone,
  parseInvitations,
  parseMe,
  parseMembers,
  type CmInvitation,
  type CmMe,
  type CmMember,
  type CmResult,
  type CmWorkspace,
} from '@/lib/clipmind/client';

type Loaded = { me: CmMe; members: CmResult<CmMember[]>; invitations: CmResult<CmInvitation[]> };

type LoadState = { kind: 'loading' } | { kind: 'failed'; failure: CmResult<never> } | { kind: 'ready'; data: Loaded };

type Action = 'switch' | 'create' | 'invite' | 'withdraw' | 'transfer' | 'expel' | 'rename' | 'delete';

const NONE: CmResult<never[]> = { ok: true, data: [] };

const section = 'flex flex-col gap-3 border-t border-border pt-6';

export function TeamView(): ReactNode {
  const router = useRouter();
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [error, setError] = useState<string | undefined>();
  const [pending, setPending] = useState<{ action: Action; id?: string } | null>(null);
  const [invite, setInvite] = useState('');
  const [newName, setNewName] = useState('');
  const [rename, setRename] = useState('');

  const load = useCallback(async (): Promise<void> => {
    const me = await cm('auth/me', { parse: parseMe });
    if (!me.ok) {
      setState({ kind: 'failed', failure: me });
      return;
    }
    const workspace = me.data.workspace;
    if (workspace === null) {
      setState({ kind: 'ready', data: { me: me.data, members: NONE, invitations: NONE } });
      return;
    }
    const [members, invitations] = await Promise.all([
      cm(`workspaces/${workspace.id}/members`, { parse: parseMembers }),
      cm(`workspaces/${workspace.id}/invitations`, { parse: parseInvitations }),
    ]);
    setRename(workspace.name);
    setState({ kind: 'ready', data: { me: me.data, members, invitations } });
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  function retry(): void {
    setState({ kind: 'loading' });
    void load();
  }

  async function run({
    action,
    id,
    call,
    done,
    after,
  }: {
    action: Action;
    id?: string;
    call: () => Promise<CmResult<true>>;
    done: string;
    after?: () => void;
  }): Promise<void> {
    setPending({ action, id });
    setError(undefined);
    const result = await call();
    if (result.ok) {
      after?.();
      toast.success(done);
      await load();
    } else {
      setError(result.message);
    }
    setPending(null);
  }

  if (state.kind === 'loading') return <LoadingText spinner>Cargando el equipo</LoadingText>;
  if (state.kind === 'failed') return <LoadErrorState error={state.failure} title="No se pudo leer tu equipo" onRetry={retry} />;

  const { me, members, invitations } = state.data;
  const workspace: CmWorkspace | null = me.workspace;
  const isOwner = workspace !== null && workspace.owner_id === me.user.id;
  const busy = pending !== null;
  const isPending = (action: Action, id?: string): boolean => pending?.action === action && pending.id === id;

  function switchTo(item: CmWorkspace): void {
    void run({
      action: 'switch',
      id: item.id,
      call: () => cm('auth/switch', { method: 'POST', body: { workspace_id: item.id }, parse: isDone }),
      done: `Ahora trabajas en ${item.name}.`,
      after: () => router.refresh(),
    });
  }

  function createWorkspace(event: FormEvent): void {
    event.preventDefault();
    const name = newName.trim();
    void run({
      action: 'create',
      call: () => cm('workspaces', { method: 'POST', body: { name: newName }, parse: isDone }),
      done: `Espacio ${name} creado.`,
      after: () => setNewName(''),
    });
  }

  function sendInvite(event: FormEvent): void {
    event.preventDefault();
    if (workspace === null) return;
    const email = invite.trim();
    void run({
      action: 'invite',
      call: () => cm(`workspaces/${workspace.id}/invitations`, { method: 'POST', body: { email: invite }, parse: isDone }),
      done: `Invitación enviada a ${email}.`,
      after: () => setInvite(''),
    });
  }

  return (
    <div className="measure-work flex flex-col gap-6">
      <FormError message={error} />

      <div className="flex flex-col gap-3">
        <h2 className="text-body-lg font-semibold text-fg-1">Tus espacios</h2>
        <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
          {me.workspaces.map((item) => (
            <li key={item.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
              <span className="min-w-0 truncate text-body-sm text-fg-1">
                {item.name} <span className="text-fg-3">{item.owner_id === me.user.id ? '· eres el dueño' : '· miembro'}</span>
              </span>
              {item.id === workspace?.id ? (
                <span className="text-body-sm text-primary">En uso</span>
              ) : (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  loading={isPending('switch', item.id)}
                  onClick={() => switchTo(item)}
                >
                  Cambiar
                </Button>
              )}
            </li>
          ))}
        </ul>
        <form onSubmit={createWorkspace} className="flex items-end gap-2">
          <Field label="Nuevo espacio" className="max-w-xs">
            {(control) => <Input {...control} value={newName} maxLength={80} onChange={(event) => setNewName(event.target.value)} required />}
          </Field>
          <Button type="submit" variant="outline" disabled={busy} loading={isPending('create')}>
            Crear
          </Button>
        </form>
      </div>

      {workspace === null ? null : (
        <>
          <div className={section}>
            <h2 className="text-body-lg font-semibold text-fg-1">Miembros de {workspace.name}</h2>
            <p className="text-body-sm text-fg-2">Todos los miembros ven y editan lo mismo. Solo el dueño puede expulsar, traspasar o borrar el espacio.</p>
            {members.ok ? (
              <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
                {members.data.map((member) => {
                  const name = accountLabel(member).name;
                  return (
                    <li key={member.user_id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                      <span className="min-w-0 truncate text-body-sm text-fg-1">
                        {name} <span className="text-fg-3">{accountLabel(member).detail}</span>
                      </span>
                      {isOwner && member.user_id !== me.user.id ? (
                        <span className="flex shrink-0 gap-2">
                          <ConfirmDialog
                            tone="default"
                            title={`¿Hacer dueño a ${name}?`}
                            description="Dejarás de ser el dueño: ya no podrás expulsar, traspasar ni borrar el espacio."
                            confirmLabel="Hacer dueño"
                            pendingLabel="Traspasando…"
                            onConfirm={() =>
                              run({
                                action: 'transfer',
                                id: member.user_id,
                                call: () => cm(`workspaces/${workspace.id}/transfer`, { method: 'POST', body: { user_id: member.user_id }, parse: isDone }),
                                done: `${name} es ahora el dueño del espacio.`,
                              })
                            }
                            trigger={
                              <Button type="button" size="sm" variant="outline" disabled={busy}>
                                Hacer dueño
                              </Button>
                            }
                          />
                          <ConfirmDialog
                            title={`¿Expulsar a ${name}?`}
                            description="Dejará de ver y editar el espacio. Puedes volver a invitarle cuando quieras."
                            confirmLabel="Expulsar"
                            pendingLabel="Expulsando…"
                            onConfirm={() =>
                              run({
                                action: 'expel',
                                id: member.user_id,
                                call: () => cm(`workspaces/${workspace.id}/members/${member.user_id}`, { method: 'DELETE', parse: isDone }),
                                done: `${name} ya no está en el espacio.`,
                              })
                            }
                            trigger={
                              <Button type="button" size="sm" variant="destructive" disabled={busy}>
                                Expulsar
                              </Button>
                            }
                          />
                        </span>
                      ) : (
                        <span className="text-body-sm text-fg-3">{member.role === 'owner' ? 'Dueño' : 'Miembro'}</span>
                      )}
                    </li>
                  );
                })}
              </ul>
            ) : (
              <LoadErrorState error={members} title="No se pudieron leer los miembros" onRetry={retry} />
            )}
          </div>

          <div className={section}>
            <h2 className="text-body-lg font-semibold text-fg-1">Invitar a alguien</h2>
            <form onSubmit={sendInvite} className="flex items-end gap-2">
              <Field label="Correo" className="max-w-xs">
                {(control) => <Input {...control} type="email" value={invite} onChange={(event) => setInvite(event.target.value)} required />}
              </Field>
              <Button type="submit" variant="outline" disabled={busy} loading={isPending('invite')}>
                Enviar invitación
              </Button>
            </form>
            {invitations.ok ? null : <LoadErrorState error={invitations} title="No se pudieron leer las invitaciones" onRetry={retry} />}
            {invitations.ok && invitations.data.length > 0 ? (
              <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
                {invitations.data.map((item) => (
                  <li key={item.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                    <span className="min-w-0 truncate text-body-sm text-fg-1">
                      {item.email} <span className="text-fg-3">· caduca el {new Date(item.expires_at).toLocaleDateString('es-ES')}</span>
                    </span>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={busy}
                      loading={isPending('withdraw', item.id)}
                      onClick={() =>
                        void run({
                          action: 'withdraw',
                          id: item.id,
                          call: () => cm(`workspaces/${workspace.id}/invitations/${item.id}`, { method: 'DELETE', parse: isDone }),
                          done: `Invitación a ${item.email} retirada.`,
                        })
                      }
                    >
                      Retirar
                    </Button>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>

          {isOwner ? (
            <div className={section}>
              <h2 className="text-body-lg font-semibold text-fg-1">Ajustes del espacio</h2>
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  const name = rename.trim();
                  void run({
                    action: 'rename',
                    call: () => cm(`workspaces/${workspace.id}`, { method: 'PATCH', body: { name: rename }, parse: isDone }),
                    done: `El espacio se llama ahora ${name}.`,
                    after: () => router.refresh(),
                  });
                }}
                className="flex items-end gap-2"
              >
                <Field label="Nombre" className="max-w-xs">
                  {(control) => <Input {...control} value={rename} maxLength={80} onChange={(event) => setRename(event.target.value)} required />}
                </Field>
                <Button type="submit" variant="outline" disabled={busy || rename.trim() === workspace.name} loading={isPending('rename')}>
                  Renombrar
                </Button>
              </form>
              <div>
                <ConfirmDialog
                  title={`¿Borrar ${workspace.name}?`}
                  description="Se borran todas sus partidas y análisis para todos los miembros. No se puede deshacer."
                  confirmLabel="Borrar el espacio"
                  pendingLabel="Borrando…"
                  onConfirm={() =>
                    run({
                      action: 'delete',
                      call: () => cm(`workspaces/${workspace.id}`, { method: 'DELETE', parse: isDone }),
                      done: `${workspace.name} borrado.`,
                      after: () => router.refresh(),
                    })
                  }
                  trigger={
                    <Button type="button" variant="destructive" disabled={busy}>
                      Borrar el espacio
                    </Button>
                  }
                />
              </div>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
