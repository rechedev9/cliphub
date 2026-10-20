'use client';

import type { ReactNode } from 'react';
import { LogIn } from 'lucide-react';
import { cloudLinkIdle } from '@/lib/cloud/account-view';
import { loginGate } from '@/lib/cloud/login-gate';
import type { CloudAccountState } from '@/lib/cloud/parse';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { Wordmark } from '@/components/brand/wordmark';
import { CloudLinkProgress, useCloudLinkFlow } from '@/components/cloud/cloud-link-flow';
import { Button } from '@/components/ui/button';

/** Studio opens only for a signed-in ClipHub account. */
export function LoginGate({ children }: { children: ReactNode }): ReactNode {
  const state = useCloudAccount();
  const gate = loginGate(state);
  if (gate === 'open') return children;
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-8 px-4 py-12">
      <Wordmark />
      {gate === 'checking' ? (
        <p role="status" className="flex items-center gap-2 text-body-sm text-fg-2">
          <span aria-hidden className="studio-spinner text-primary" />
          Comprobando tu cuenta…
        </p>
      ) : (
        <SignIn state={state} />
      )}
    </main>
  );
}

function SignIn({ state }: { state: CloudAccountState }): ReactNode {
  const flow = useCloudLinkFlow({ active: true, openPending: false });
  const idle = cloudLinkIdle(state) && !flow.busy && flow.phase.kind === 'starting';
  return (
    <section aria-labelledby="sign-in-title" className="studio-panel studio-panel-raised flex w-full max-w-[30rem] flex-col gap-5 p-7">
      <div className="flex flex-col gap-3">
        <SectionEyebrow label="Cuenta de ClipHub" />
        <h1 id="sign-in-title" className="font-display text-title font-bold text-fg-1">
          Inicia sesión para usar Studio
        </h1>
        <p className="text-body text-fg-2">
          Entra con tu cuenta en la web de ClipHub, en tu navegador. Después podrás crear Shorts y vídeos largos en este PC o en la nube.
        </p>
      </div>
      {idle ? (
        <Button type="button" variant="hero" size="sm" className="self-start" onClick={flow.start}>
          <LogIn aria-hidden />
          Iniciar sesión
        </Button>
      ) : (
        <CloudLinkProgress flow={flow} />
      )}
    </section>
  );
}
