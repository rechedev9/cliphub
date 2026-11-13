'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { cm, isDone, safeNext } from '@/lib/clipmind/client';

interface FormShellProps {
  title: string;
  lead: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
}

function FormShell({ title, lead, children, footer }: FormShellProps): ReactNode {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1.5">
        <h1 className="text-title font-semibold text-fg-1">{title}</h1>
        <p className="text-body-sm text-fg-2">{lead}</p>
      </div>
      {children}
      {footer ? <p className="text-body-sm text-fg-3">{footer}</p> : null}
    </div>
  );
}

export function FormError({ message }: { message: string | undefined }): ReactNode {
  return message ? (
    <p role="alert" className="text-body-sm text-destructive">
      {message}
    </p>
  ) : null;
}

const link = 'text-primary underline-offset-4 hover:underline';

export function LoginForm(): ReactNode {
  const router = useRouter();
  const params = useSearchParams();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const next = safeNext(params.get('next'));

  async function submit(event: FormEvent): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(undefined);
    const result = await cm('auth/login', { method: 'POST', body: { email, password, device: 'Navegador' }, parse: isDone });
    if (result.ok) {
      router.replace(next);
      router.refresh();
      return;
    }
    setError(result.message);
    setBusy(false);
  }

  return (
    <FormShell
      title="Entra en ClipMind"
      lead="Analiza tus partidas de CS2 con tu equipo."
      footer={
        <>
          ¿Sin cuenta? <Link href="/crear-cuenta" className={link}>Crea una</Link>
        </>
      }
    >
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <Field label="Correo" required>
          {(control) => <Input {...control} type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required />}
        </Field>
        <Field label="Contraseña" required hint={<Link href="/recuperar" className={link}>He olvidado mi contraseña</Link>}>
          {(control) => (
            <Input {...control} type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
          )}
        </Field>
        <FormError message={error} />
        <Button type="submit" loading={busy}>
          Entrar
        </Button>
      </form>
    </FormShell>
  );
}

export function SignupForm(): ReactNode {
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);

  async function submit(event: FormEvent): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(undefined);
    const result = await cm('auth/signup', { method: 'POST', body: { email, password, display_name: name }, parse: isDone });
    if (result.ok) setSent(true);
    else setError(result.message);
    setBusy(false);
  }

  if (sent) {
    return (
      <FormShell title="Revisa tu correo" lead={`Te hemos enviado un enlace a ${email} para confirmar la cuenta.`}>
        <Link href="/iniciar-sesion" className={link}>
          Ir a entrar
        </Link>
      </FormShell>
    );
  }
  return (
    <FormShell
      title="Crea tu cuenta"
      lead="Tu equipo y tú compartís el mismo espacio de análisis."
      footer={
        <>
          ¿Ya tienes cuenta? <Link href="/iniciar-sesion" className={link}>Entra</Link>
        </>
      }
    >
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <Field label="Nombre" hint="Cómo te verá tu equipo.">
          {(control) => <Input {...control} autoComplete="name" value={name} onChange={(event) => setName(event.target.value)} maxLength={80} />}
        </Field>
        <Field label="Correo" required>
          {(control) => <Input {...control} type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required />}
        </Field>
        <Field label="Contraseña" required hint="Al menos 10 caracteres.">
          {(control) => (
            <Input {...control} type="password" autoComplete="new-password" minLength={10} value={password} onChange={(event) => setPassword(event.target.value)} required />
          )}
        </Field>
        <FormError message={error} />
        <Button type="submit" loading={busy}>
          Crear cuenta
        </Button>
      </form>
    </FormShell>
  );
}

export function ForgotForm(): ReactNode {
  const [email, setEmail] = useState('');
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);

  async function submit(event: FormEvent): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(undefined);
    const result = await cm('auth/forgot', { method: 'POST', body: { email }, parse: isDone });
    if (result.ok) setSent(true);
    else setError(result.message);
    setBusy(false);
  }

  if (sent) {
    return (
      <FormShell title="Revisa tu correo" lead="Si hay una cuenta con ese correo, te hemos enviado un enlace para elegir otra contraseña.">
        <Link href="/iniciar-sesion" className={link}>
          Volver a entrar
        </Link>
      </FormShell>
    );
  }
  return (
    <FormShell title="Recupera tu contraseña" lead="Te enviaremos un enlace para elegir una nueva.">
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <Field label="Correo" required>
          {(control) => <Input {...control} type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required />}
        </Field>
        <FormError message={error} />
        <Button type="submit" loading={busy}>
          Enviar enlace
        </Button>
      </form>
    </FormShell>
  );
}

export function ResetForm(): ReactNode {
  const router = useRouter();
  const token = useSearchParams().get('token') ?? '';
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(undefined);
    const result = await cm('auth/reset', { method: 'POST', body: { token, password }, parse: isDone });
    if (result.ok) {
      router.replace('/iniciar-sesion');
      return;
    }
    setError(result.message);
    setBusy(false);
  }

  return (
    <FormShell title="Elige una contraseña nueva" lead="Cerraremos tus otras sesiones al cambiarla.">
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <Field label="Contraseña nueva" required hint="Al menos 10 caracteres.">
          {(control) => (
            <Input {...control} type="password" autoComplete="new-password" minLength={10} value={password} onChange={(event) => setPassword(event.target.value)} required />
          )}
        </Field>
        <FormError message={error} />
        <Button type="submit" loading={busy} disabled={token === ''}>
          Cambiar contraseña
        </Button>
      </form>
    </FormShell>
  );
}

/** Confirms the email behind the link; the password proves who signed up. */
export function ConfirmView(): ReactNode {
  const token = useSearchParams().get('token') ?? '';
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  async function submit(event: FormEvent): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(undefined);
    const result = await cm('auth/confirm', { method: 'POST', body: { token, password }, parse: isDone });
    if (result.ok) setDone(true);
    else setError(result.code === 'invalid_credentials' ? 'Esa no es la contraseña con la que creaste la cuenta.' : result.message);
    setBusy(false);
  }

  if (token === '') {
    return (
      <FormShell title="No se pudo confirmar" lead="El enlace no es válido o ha caducado.">
        <Link href="/crear-cuenta" className={link}>
          Crear la cuenta de nuevo
        </Link>
      </FormShell>
    );
  }
  if (done) {
    return (
      <FormShell title="Correo confirmado" lead="Tu cuenta está lista.">
        <Link href="/iniciar-sesion" className={link}>
          Entrar
        </Link>
      </FormShell>
    );
  }
  return (
    <FormShell title="Confirma tu correo" lead="Escribe la contraseña que elegiste al crear la cuenta.">
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <Field label="Contraseña" required>
          {(control) => (
            <Input {...control} type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
          )}
        </Field>
        <FormError message={error} />
        <Button type="submit" loading={busy}>
          Confirmar
        </Button>
      </form>
    </FormShell>
  );
}

/** Accepts a team invitation for the signed-in person; asks them to sign in first otherwise. */
export function InvitationView(): ReactNode {
  const router = useRouter();
  const token = useSearchParams().get('token') ?? '';
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const here = `/invitacion?token=${encodeURIComponent(token)}`;

  async function accept(): Promise<void> {
    setBusy(true);
    setError(undefined);
    const result = await cm('invitations/accept', { method: 'POST', body: { token }, parse: isDone });
    if (result.ok) {
      router.replace('/equipo');
      router.refresh();
      return;
    }
    if (result.code === 'unauthorized') {
      router.replace(`/iniciar-sesion?next=${encodeURIComponent(here)}`);
      return;
    }
    setError(result.message);
    setBusy(false);
  }

  return (
    <FormShell title="Te han invitado a un equipo" lead="Al unirte verás las partidas y el análisis que comparte el equipo, y podrás editarlos.">
      <FormError message={error} />
      <Button type="button" loading={busy} disabled={token === ''} onClick={() => void accept()}>
        Unirme al equipo
      </Button>
    </FormShell>
  );
}
