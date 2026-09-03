'use client';

import type { ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { setPendingDemoFiles } from '@/lib/clips/pending-upload';
import { HUB_EMPTY_TITLE } from '@/lib/clips/copy';
import { NEW_DEMO_HREF } from '@/lib/clips/routes';
import { DemoDropzone } from '@/components/upload/demo-dropzone';

/** First-run hub: the dropzone is the only door, and it hands its files to /clips/nueva. */
export function HubEmpty(): ReactNode {
  const router = useRouter();
  return (
    <section aria-label={HUB_EMPTY_TITLE} className="studio-enter flex max-w-[1080px] flex-col gap-5">
      <div className="flex flex-col gap-2">
        <h1 className="font-display text-display font-bold uppercase text-fg-1">{HUB_EMPTY_TITLE}</h1>
        <p className="max-w-[640px] text-body text-fg-2">
          Un .dem de CS2. Lo parseamos en este PC, eliges tu POV y sacas Shorts o el Full POV. Sin login, nada sale de
          tu equipo.
        </p>
      </div>
      <DemoDropzone
        minHeightClass="min-h-[240px]"
        onFiles={(files) => {
          setPendingDemoFiles(files);
          router.push(NEW_DEMO_HREF);
        }}
      />
    </section>
  );
}
