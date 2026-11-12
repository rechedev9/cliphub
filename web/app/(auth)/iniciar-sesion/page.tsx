import { Suspense, type ReactElement } from 'react';
import type { Metadata } from 'next';
import { LoginForm } from '@/components/clipmind/auth-forms';

export const metadata: Metadata = { title: 'Entrar' };

export default function Page(): ReactElement {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}
