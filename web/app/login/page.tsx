import type { ReactElement } from 'react';
import { AuthForm } from '@/components/account/auth-form';
import { AuthShell } from '@/components/account/auth-shell';

interface LoginPageProps {
  searchParams: Promise<{ next?: string; error?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps): Promise<ReactElement> {
  const query = await searchParams;
  return (
    <AuthShell
      title="Continúa en tu PC"
      description={query.error === 'service'
        ? 'El servicio de cuentas no responde ahora mismo. Puedes volver a intentarlo en unos segundos.'
        : 'Inicia sesión para conectar este equipo y abrir tu Studio en Chrome.'}
    >
      <AuthForm mode="login" nextPath={query.next ?? '/connect'} />
    </AuthShell>
  );
}
