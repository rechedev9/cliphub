import { Suspense, type ReactElement } from 'react';
import type { Metadata } from 'next';
import { SignupForm } from '@/components/clipmind/auth-forms';

export const metadata: Metadata = { title: 'Crear cuenta' };

export default function Page(): ReactElement {
  return (
    <Suspense>
      <SignupForm />
    </Suspense>
  );
}
