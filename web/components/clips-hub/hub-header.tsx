import type { ReactNode } from 'react';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { HUB_DESCRIPTION, HUB_LOAD_DEMO_CTA, HUB_TITLE } from '@/lib/clips/copy';
import { NEW_DEMO_HREF, type HubLens } from '@/lib/clips/routes';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';

/** The H1 names what is on screen: the clips lens lists outputs, not partidas. */
const LENS_COPY = {
  partidas: { title: HUB_TITLE, description: HUB_DESCRIPTION },
  clips: {
    title: 'Tus vídeos de demos',
    description: 'Shorts y vídeos largos creados desde tus partidas. Descarga el MP4 o prepara su publicación.',
  },
} as const satisfies Record<HubLens, { title: string; description: string }>;

export type HubHeaderProps = {
  lens: HubLens;
};

/** Hub title block: the one primary door, Cargar demo, lives here rather than in the strip. */
export function HubHeader({ lens }: HubHeaderProps): ReactNode {
  const copy = LENS_COPY[lens];
  return (
    <StudioPageHeader
      title={copy.title}
      description={copy.description}
      actions={
        <Button asChild variant="hero" className="neon-notch">
          <Link href={NEW_DEMO_HREF}>
            <Plus aria-hidden />
            {HUB_LOAD_DEMO_CTA}
          </Link>
        </Button>
      }
    />
  );
}
