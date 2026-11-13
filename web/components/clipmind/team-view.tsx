'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { FormError } from '@/components/clipmind/auth-forms';
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
  type CmWorkspace,
} from '@/lib/clipmind/client';

interface Loaded {
  me: CmMe;
  members: CmMember[];
  invitations: CmInvitation[];
}

const section = 'flex flex-col gap-3 border-t border-border pt-6';

/** The team workspace: who is in it, invitations, and the few things only its owner can do. */
export function TeamView(): ReactNode {
  const router = useRouter();
  const [data, setData] = useState<Loaded | undefined>();
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [invite, setInvite] = useState('');
  const [newName, setNewName] = useState('');
  const [rename, setRename] = useState('');

  const load = useCallback(async (): Promise<void> => {
    const me = await cm('auth/me', { parse: parseMe });
    if (!me.ok) {
      setError(me.message);
      return;
    }
    const workspace = me.data.workspace;
    if (workspace === null) {
      setData({ me: me.data, members: [], invitations: [] });
      return;
    }
    const [members, invitations] = await Promise.all([
      cm(`workspaces/${workspace.id}/members`, { parse: parseMembers }),
      cm(`workspaces/${workspace.id}/invitations`, { parse: parseInvitations }),
    ]);
    setRename(workspace.name);
    setData({ me: me.data, members: members.ok ? members.data : [], invitations: invitations.ok ? invitations.data : [] });
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function run(action: () => Promise<{ ok: true } | { ok: false; message: string }>, after?: () => void): Promise<void> {
    setBusy(true);
    setError(undefined);
    const result = await action();
    if (result.ok) {
      after?.();
      await load();
    } else {
      setError(result.message);
    }
    setBusy(false);
  }

  if (data === undefined) {
    return error ? <FormError message={error} /> : <p className="text-body-sm text-fg-3">Cargando…</p>;
  }
  const { me, members, invitations } = data;
  const workspace: CmWorkspace | null = me.workspace;
  const isOwner = workspace !== null && workspace.owner_id === me.user.id;

  async function switchTo(id: string): Promise<void> {
    await run(
      () => cm('auth/switch', { method: 'POST', body: { workspace_id: id }, parse: isDone }),
      () => router.refresh(),
    );
  }

  async function createWorkspace(event: FormEvent): Promise<void> {
    event.preventDefault();
    await run(
      () => cm('workspaces', { method: 'POST', body: { name: newName }, parse: isDone }),
      () => setNewName(''),
    );
  }

  async function sendInvite(event: FormEvent): Promise<void> {
    event.preventDefault();
    if (workspace === null) return;
    await run(
      () => cm(`workspaces/${workspace.id}/invitations`, { method: 'POST', body: { email: invite }, parse: isDone }),
      () => setInvite(''),
    );
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
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void switchTo(item.id)}>
                  Cambiar
                </Button>
              )}
            </li>
          ))}
        </ul>
        <form onSubmit={(event) => void createWorkspace(event)} className="flex items-end gap-2">
          <Field label="Nuevo espacio" className="max-w-xs">
            {(control) => <Input {...control} value={newName} maxLength={80} onChange={(event) => setNewName(event.target.value)} required />}
          </Field>
          <Button type="submit" variant="outline" disabled={busy}>
            Crear
          </Button>
        </form>
      </div>

      {workspace === null ? null : (
        <>
          <div className={section}>
            <h2 className="text-body-lg font-semibold text-fg-1">Miembros de {workspace.name}</h2>
            <p className="text-body-sm text-fg-2">Todos los miembros ven y editan lo mismo. Solo el dueño puede expulsar, traspasar o borrar el espacio.</p>
            <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
              {members.map((member) => (
                <li key={member.user_id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                  <span className="min-w-0 truncate text-body-sm text-fg-1">
                    {accountLabel(member).name} <span className="text-fg-3">{accountLabel(member).detail}</span>
                  </span>
                  {isOwner && member.user_id !== me.user.id ? (
                    <span className="flex shrink-0 gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={busy}
                        onClick={() => {
                          if (window.confirm(`¿Traspasar el espacio a ${accountLabel(member).name}? Dejarás de ser el dueño.`)) {
                            void run(() => cm(`workspaces/${workspace.id}/transfer`, { method: 'POST', body: { user_id: member.user_id }, parse: isDone }));
                          }
                        }}
                      >
                        Hacer dueño
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="destructive"
                        disabled={busy}
                        onClick={() => {
                          if (window.confirm(`¿Expulsar a ${accountLabel(member).name} del espacio?`)) {
                            void run(() => cm(`workspaces/${workspace.id}/members/${member.user_id}`, { method: 'DELETE', parse: isDone }));
                          }
                        }}
                      >
                        Expulsar
                      </Button>
                    </span>
                  ) : (
                    <span className="text-body-sm text-fg-3">{member.role === 'owner' ? 'Dueño' : 'Miembro'}</span>
                  )}
                </li>
              ))}
            </ul>
          </div>

          <div className={section}>
            <h2 className="text-body-lg font-semibold text-fg-1">Invitar a alguien</h2>
            <form onSubmit={(event) => void sendInvite(event)} className="flex items-end gap-2">
              <Field label="Correo" className="max-w-xs">
                {(control) => <Input {...control} type="email" value={invite} onChange={(event) => setInvite(event.target.value)} required />}
              </Field>
              <Button type="submit" variant="outline" disabled={busy}>
                Enviar invitación
              </Button>
            </form>
            {invitations.length > 0 ? (
              <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
                {invitations.map((item) => (
                  <li key={item.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                    <span className="min-w-0 truncate text-body-sm text-fg-1">
                      {item.email} <span className="text-fg-3">· caduca el {new Date(item.expires_at).toLocaleDateString('es-ES')}</span>
                    </span>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={busy}
                      onClick={() => void run(() => cm(`workspaces/${workspace.id}/invitations/${item.id}`, { method: 'DELETE', parse: isDone }))}
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
                  void run(() => cm(`workspaces/${workspace.id}`, { method: 'PATCH', body: { name: rename }, parse: isDone }), () => router.refresh());
                }}
                className="flex items-end gap-2"
              >
                <Field label="Nombre" className="max-w-xs">
                  {(control) => <Input {...control} value={rename} maxLength={80} onChange={(event) => setRename(event.target.value)} required />}
                </Field>
                <Button type="submit" variant="outline" disabled={busy}>
                  Renombrar
                </Button>
              </form>
              <div>
                <Button
                  type="button"
                  variant="destructive"
                  disabled={busy}
                  onClick={() => {
                    if (window.confirm(`¿Borrar "${workspace.name}" con todas sus partidas y análisis? No se puede deshacer.`)) {
                      void run(
                        () => cm(`workspaces/${workspace.id}`, { method: 'DELETE', parse: isDone }),
                        () => router.refresh(),
                      );
                    }
                  }}
                >
                  Borrar el espacio
                </Button>
              </div>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
