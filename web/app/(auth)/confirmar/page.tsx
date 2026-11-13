import { Suspense, type ReactElement } from 'react';
import type { Metadata } from 'next';
import { ConfirmView } from '@/components/clipmind/auth-forms';

export const metadata: Metadata = { title: 'Confirmar correo' };

export default function Page(): ReactElement {
  return (
    <Suspense>
      <ConfirmView />
    </Suspense>
  );
}
