import { Suspense, type ReactElement } from 'react';
import type { Metadata } from 'next';
import { ForgotForm } from '@/components/clipmind/auth-forms';

export const metadata: Metadata = { title: 'Recuperar contraseña' };

export default function Page(): ReactElement {
  return (
    <Suspense>
      <ForgotForm />
    </Suspense>
  );
}
