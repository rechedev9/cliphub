'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { setPendingDemoFiles } from '@/lib/clips/pending-upload';
import { HUB_EMPTY_TITLE } from '@/lib/clips/copy';
import { NEW_DEMO_HREF } from '@/lib/clips/routes';
import { StudioPageHeader } from '@/components/studio/page-header';
import { DemoDropzone } from '@/components/upload/demo-dropzone';
import { CreationPaths } from '@/components/studio/creation-paths';
import { DemoSourceHelp } from '@/components/onboarding/demo-source-help';
import { FirstRunGuide } from '@/components/clips-hub/first-run-guide';
import { FOCUS_RING } from '@/components/ui/button';

const HUB_EMPTY_DESCRIPTION =
  'Convierte tus partidas de CS2 o grabaciones de stream en vídeos. Elige un formato para empezar; revisarás el contenido antes de grabar.';

/** Direct creation choices; the dropzone also accepts files without a prior format choice. */
export function HubEmpty(): ReactNode {
  const router = useRouter();
  return (
    <section aria-label={HUB_EMPTY_TITLE} className="measure-list flex flex-col gap-5">
      <StudioPageHeader title={HUB_EMPTY_TITLE} description={HUB_EMPTY_DESCRIPTION} />
      <CreationPaths />
      <FirstRunGuide progress={{ load: false, pick: false, produce: false }} />
      <div className="flex flex-col gap-3">
        <h2 className="text-body font-semibold text-fg-1">¿Ya tienes una demo de CS2?</h2>
        <DemoDropzone
          compact
          onFiles={(files) => {
            setPendingDemoFiles(files);
            router.push(NEW_DEMO_HREF);
          }}
        />
        <DemoSourceHelp />
      </div>
      <p className="text-body-sm text-fg-3">
        Para grabar demos necesitas ClipHub Studio en Windows, CS2 y HLAE. Puedes preparar tu vídeo antes de grabar.{' '}
        <Link href="/settings#capture" className={`text-primary underline underline-offset-4 ${FOCUS_RING}`}>Revisar requisitos de grabación</Link>.
        {' '}Los clips de stream no necesitan CS2 ni HLAE.
      </p>
    </section>
  );
}
