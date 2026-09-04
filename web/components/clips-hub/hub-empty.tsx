'use client';

import type { ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { setPendingDemoFiles } from '@/lib/clips/pending-upload';
import { HUB_EMPTY_TITLE } from '@/lib/clips/copy';
import { FIRST_RUN_NONE } from '@/lib/clips/hub';
import { NEW_DEMO_HREF } from '@/lib/clips/routes';
import { StudioPageHeader } from '@/components/studio/page-header';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { FirstRunGuide } from '@/components/clips-hub/first-run-guide';

const HUB_EMPTY_DESCRIPTION =
  'Un .dem de CS2. Lo parseamos en este PC, eliges tu POV y sacas Shorts o el Full POV. Sin login, nada sale de tu equipo.';

/** First-run hub: the dropzone is the only door, and it hands its files to /clips/nueva. */
export function HubEmpty(): ReactNode {
  const router = useRouter();
  return (
    <section aria-label={HUB_EMPTY_TITLE} className="measure-list flex flex-col gap-5">
      <StudioPageHeader title={HUB_EMPTY_TITLE} description={HUB_EMPTY_DESCRIPTION} />
      <DemoDropzone
        minHeightClass="min-h-[240px]"
        onFiles={(files) => {
          setPendingDemoFiles(files);
          router.push(NEW_DEMO_HREF);
        }}
      />
      <FirstRunGuide progress={FIRST_RUN_NONE} />
    </section>
  );
}
