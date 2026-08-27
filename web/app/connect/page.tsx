import { Suspense, type ReactElement } from 'react';
import { ConnectWorkstation } from '@/components/agent/connect-workstation';

export default function ConnectPage(): ReactElement {
  return (
    <Suspense fallback={<main className="min-h-screen" aria-label="Conectando ClipHub Agent" />}>
      <ConnectWorkstation />
    </Suspense>
  );
}
