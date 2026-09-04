'use client';

import type { ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { setPendingDemoFiles } from '@/lib/clips/pending-upload';
import { HUB_EMPTY_TITLE } from '@/lib/clips/copy';
import { NEW_DEMO_HREF } from '@/lib/clips/routes';
import { StudioPageHeader } from '@/components/studio/page-header';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { CreationPaths } from '@/components/studio/creation-paths';

const HUB_EMPTY_DESCRIPTION =
  'Elige qué quieres crear. Tus demos, grabaciones y vídeos se procesan en este PC, sin cuenta.';

/** Direct creation choices; the dropzone also accepts files without a prior format choice. */
export function HubEmpty(): ReactNode {
  const router = useRouter();
  return (
    <section aria-label={HUB_EMPTY_TITLE} className="measure-list flex flex-col gap-5">
      <StudioPageHeader title={HUB_EMPTY_TITLE} description={HUB_EMPTY_DESCRIPTION} />
      <CreationPaths />
      <h2 className="text-body font-semibold text-fg-1">¿Ya tienes una demo? Cárgala aquí</h2>
      <DemoDropzone
        minHeightClass="min-h-[160px]"
        onFiles={(files) => {
          setPendingDemoFiles(files);
          router.push(NEW_DEMO_HREF);
        }}
      />

    </section>
  );
}
