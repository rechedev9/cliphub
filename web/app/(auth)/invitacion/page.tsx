import { Suspense, type ReactElement } from 'react';
import type { Metadata } from 'next';
import { InvitationView } from '@/components/clipmind/auth-forms';

export const metadata: Metadata = { title: 'Invitación' };

export default function Page(): ReactElement {
  return (
    <Suspense>
      <InvitationView />
    </Suspense>
  );
}
