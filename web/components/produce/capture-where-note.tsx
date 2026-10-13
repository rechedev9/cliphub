'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Cloud, Monitor, type LucideIcon } from 'lucide-react';
import { CLOUD_HREF, PRODUCE_FORMAT, type ProduceFormat } from '@/lib/clips/routes';
import { cloudConnectionTag } from '@/lib/cloud/account-view';
import { CAPTURE_LOCAL_ONLY_HINT, CAPTURE_TARGET_HINT, CAPTURE_TARGET_LABEL } from '@/lib/cloud/capture-target';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';

const WHEN_HINT = {
  short: 'Lo eliges al preparar el Short, justo antes de crearlo.',
  full: 'Los vídeos largos se graban siempre en este PC.',
} as const satisfies Record<ProduceFormat, string>;

/** Flow home: where this format can be recorded, before the user loads anything. */
export function CaptureWhereNote({ format }: { format: ProduceFormat }): ReactNode {
  const state = useCloudAccount();
  const tag = cloudConnectionTag(state);
  const short = format === PRODUCE_FORMAT.short;
  const offerConnect = short && state.kind === 'ready' && !state.account.linked;

  return (
    <section aria-labelledby="capture-where-title" className="studio-panel flex flex-col gap-3 p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h2 id="capture-where-title" className="font-mono text-meta uppercase tracking-widest text-fg-3">
          Dónde se graba
        </h2>
        <p className="text-body-sm text-fg-2">{WHEN_HINT[format]}</p>
      </div>
      <div className="grid gap-3 @[44rem]/content:grid-cols-2">
        <Place icon={Monitor} title={CAPTURE_TARGET_LABEL.local} hint={CAPTURE_TARGET_HINT.local} />
        {short ? (
          <Place
            icon={Cloud}
            title={CAPTURE_TARGET_LABEL.cloud}
            hint={CAPTURE_TARGET_HINT.cloud}
            status={tag === null ? null : <StatusTag tone={tag.tone}>{tag.text}</StatusTag>}
            action={
              offerConnect ? (
                <Button asChild size="xs" variant="outline-primary">
                  <Link href={CLOUD_HREF}>Conectar con la nube</Link>
                </Button>
              ) : null
            }
          />
        ) : (
          <Place
            muted
            icon={Cloud}
            title={CAPTURE_TARGET_LABEL.cloud}
            hint={CAPTURE_LOCAL_ONLY_HINT}
            status={<StatusTag tone="neutral">Solo Shorts</StatusTag>}
          />
        )}
      </div>
    </section>
  );
}

type PlaceProps = {
  icon: LucideIcon;
  title: string;
  hint: string;
  status?: ReactNode;
  action?: ReactNode;
  muted?: boolean;
};

function Place({ icon: Icon, title, hint, status = null, action = null, muted = false }: PlaceProps): ReactNode {
  return (
    <div className="flex min-w-0 items-start gap-3 rounded-md border border-border-subtle bg-surface-1 px-3 py-2.5">
      <Icon aria-hidden className={muted ? 'mt-1 size-5 shrink-0 text-fg-3' : 'mt-1 size-5 shrink-0 text-primary'} />
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-h-7 flex-wrap items-center gap-2">
          <span className="text-body-sm font-semibold text-fg-1">{title}</span>
          {status}
        </div>
        <p className="text-body-sm text-fg-2">{hint}</p>
        {action === null ? null : <div>{action}</div>}
      </div>
    </div>
  );
}
