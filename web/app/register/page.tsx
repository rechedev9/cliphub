import type { ReactElement } from 'react';
import { AuthForm } from '@/components/account/auth-form';
import { AuthShell } from '@/components/account/auth-shell';

interface RegisterPageProps {
  searchParams: Promise<{ next?: string }>;
}

export default async function RegisterPage({ searchParams }: RegisterPageProps): Promise<ReactElement> {
  const query = await searchParams;
  return (
    <AuthShell
      title="Crea tu espacio local"
      description="Tu cuenta identifica tus equipos. El almacenamiento y el procesamiento continúan dentro de cada PC."
    >
      <AuthForm mode="register" nextPath={query.next ?? '/connect'} />
    </AuthShell>
  );
}
