import type { ReactNode } from 'react';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { HUB_DESCRIPTION, HUB_LOAD_DEMO_CTA, HUB_TITLE } from '@/lib/clips/copy';
import { NEW_DEMO_HREF } from '@/lib/clips/routes';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';

/** Hub title block: the one primary door, Cargar demo, lives here rather than in the strip. */
export function HubHeader(): ReactNode {
  return (
    <StudioPageHeader
      title={HUB_TITLE}
      description={HUB_DESCRIPTION}
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
