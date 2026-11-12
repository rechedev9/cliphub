import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { AccountView } from '@/components/clipmind/account-view';

export const metadata: Metadata = { title: 'Cuenta' };

export default function Page(): ReactNode {
  return (
    <div className="flex flex-col gap-7">
      <StudioPageHeader title="Cuenta" description="Tu sesión, las claves para asistentes y tus datos." />
      <AccountView />
    </div>
  );
}
