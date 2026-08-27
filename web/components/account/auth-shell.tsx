import type { ReactElement, ReactNode } from 'react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { Wordmark } from '@/components/brand/wordmark';

interface AuthShellProps {
  title: string;
  description: string;
  children: ReactNode;
}

export function AuthShell({ title, description, children }: AuthShellProps): ReactElement {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-[32rem] flex-col justify-center gap-8 px-6 py-12">
      <Wordmark />
      <section className="studio-panel studio-panel-raised flex flex-col gap-6 p-7 sm:p-9">
        <div className="flex flex-col gap-3">
          <SectionEyebrow label="CUENTA CLIPHUB" />
          <h1 className="font-display text-display-sm font-bold text-fg-1">{title}</h1>
          <p className="text-body text-fg-2">{description}</p>
        </div>
        {children}
      </section>
      <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
        Tus demos y vídeos permanecen en tu PC.
      </p>
    </main>
  );
}
