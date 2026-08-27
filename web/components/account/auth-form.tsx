'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState, type FormEvent, type ReactElement } from 'react';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { loginAccount, registerAccount } from '@/lib/api/account';

interface AuthFormProps {
  mode: 'login' | 'register';
  nextPath: string;
}

export function AuthForm({ mode, nextPath }: AuthFormProps): ReactElement {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const result = mode === 'login'
        ? await loginAccount(email, password)
        : await registerAccount(email, password);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      router.replace(safeNextPath(nextPath));
      router.refresh();
    } catch {
      setError('No se pudo conectar con ClipHub. Inténtalo de nuevo.');
    } finally {
      setBusy(false);
    }
  }

  const login = mode === 'login';
  return (
    <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-5">
      <Field label="Correo" required>
        {(control) => (
          <Input
            {...control}
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        )}
      </Field>
      <Field
        label="Contraseña"
        hint={login ? undefined : '12 caracteres como mínimo.'}
        error={error}
        required
      >
        {(control) => (
          <Input
            {...control}
            type="password"
            autoComplete={login ? 'current-password' : 'new-password'}
            minLength={12}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        )}
      </Field>
      <Button type="submit" variant="hero" size="lg" loading={busy} loadingText="CONECTANDO…">
        {login ? 'ENTRAR' : 'CREAR CUENTA'}
      </Button>
      <p className="text-center text-body-sm text-fg-2">
        {login ? '¿Todavía no tienes cuenta?' : '¿Ya tienes una cuenta?'}{' '}
        <Link
          className="font-semibold text-primary hover:underline"
          href={`${login ? '/register' : '/login'}?next=${encodeURIComponent(safeNextPath(nextPath))}`}
        >
          {login ? 'Crear cuenta' : 'Entrar'}
        </Link>
      </p>
    </form>
  );
}

function safeNextPath(value: string): string {
  return value.startsWith('/') && !value.startsWith('//') ? value : '/connect';
}
