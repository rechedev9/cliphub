import type { ReactNode } from 'react';
import guideSource from '@/content/algoritmos.md';
import { parseMarkdown } from '@/lib/algorithms/markdown';
import { MarkdownBlocks } from '@/components/markdown/markdown-blocks';
import { StudioPageHeader } from '@/components/studio/page-header';

const PAGE_TITLE = 'Consejos para publicar';
const PAGE_DESCRIPTION = 'Cómo montar, titular y subir tus Shorts y demos completas para que lleguen a más gente en YouTube.';

const BLOCKS = parseMarkdown(guideSource);

export function AlgorithmsGuide(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-8">
      <StudioPageHeader title={PAGE_TITLE} description={PAGE_DESCRIPTION} />

      <article className="flex flex-col gap-4">
        <MarkdownBlocks blocks={BLOCKS} />
      </article>
    </div>
  );
}
