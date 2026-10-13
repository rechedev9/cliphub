'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Cloud } from 'lucide-react';
import { CLOUD_HREF } from '@/lib/clips/routes';
import { cloudConnectionTag } from '@/lib/cloud/account-view';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

/** Settings row for the cloud account: its state and the door to the cloud section, which owns the connection. */
export function CloudAccountCard(): ReactNode {
  const state = useCloudAccount();
  const tag = cloudConnectionTag(state);
  const linked = state.kind === 'ready' && state.account.linked;

  return (
    <section id="cloud" className="studio-panel flex scroll-mt-20 flex-col gap-5 p-4 @[34rem]/content:p-6" aria-labelledby="cloud-account-title">
      <div className="flex items-center gap-4">
        <IconTile icon={Cloud} size="md" depth="inset" />
        <div className="flex min-w-0 flex-col gap-1">
          <SectionEyebrow label="GRABAR SIN USAR ESTE PC" />
          <h2 id="cloud-account-title" className="font-display text-title font-bold uppercase text-fg-1">
            Nube ClipHub
          </h2>
        </div>
        <div className="ml-auto shrink-0">
          {tag === null ? (
            <Skeleton aria-label="Comprobando la cuenta de ClipHub" className="h-7 w-32 rounded-none" />
          ) : (
            <StatusTag tone={tag.tone}>{tag.text}</StatusTag>
          )}
        </div>
      </div>
      <p className="text-body-sm text-fg-2">
        Al crear un Short puedes elegir que lo grabe un servidor de ClipHub en lugar de este PC. La conexión de la cuenta, el
        uso y la cola están en la sección Nube.
      </p>
      <div className="flex flex-wrap gap-3">
        <Button asChild variant={linked ? 'outline' : 'default'}>
          <Link href={CLOUD_HREF}>{linked ? 'Ver la cuenta en Nube' : 'Conectar en Nube'}</Link>
        </Button>
      </div>
    </section>
  );
}
