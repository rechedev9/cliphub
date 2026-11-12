import type { ReactElement, ReactNode } from 'react';

/** The signed-out screens of the web edition: one centered column, no app shell. */
export default function AuthLayout({ children }: { children: ReactNode }): ReactElement {
  return (
    <main className="flex min-h-dvh items-center justify-center px-4 py-10">
      <div className="w-full max-w-sm">
        <p className="mb-8 text-label font-semibold tracking-wider text-primary uppercase">ClipMind</p>
        {children}
      </div>
    </main>
  );
}
