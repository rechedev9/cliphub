import { Suspense, type ReactElement } from 'react';
import type { Metadata } from 'next';
import { ResetForm } from '@/components/clipmind/auth-forms';

export const metadata: Metadata = { title: 'Nueva contraseña' };

export default function Page(): ReactElement {
  return (
    <Suspense>
      <ResetForm />
    </Suspense>
  );
}
