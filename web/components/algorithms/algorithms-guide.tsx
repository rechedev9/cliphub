import type { ReactNode } from 'react';
import guideSource from '@/content/algoritmos.md';
import { parseMarkdown, type Block, type EvidenceLevel, type Inline } from '@/lib/algorithms/markdown';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { StudioPageHeader } from '@/components/studio/page-header';
import { STATUS_TONE_CLASS } from '@/components/studio/status-tag';

const PAGE_TITLE = 'Ideas';
const PAGE_DESCRIPTION =
  'Cómo deciden YouTube y Google qué vídeos recomiendan, qué mide cada plataforma mientras ves un vídeo y qué hacer con tus Shorts y vídeos largos de CS2.';

const BLOCKS = parseMarkdown(guideSource);
const CONTENTS = BLOCKS.flatMap((block) => (block.kind === 'heading' && block.level === 2 ? [block] : []));

const EVIDENCE_META = {
  R: { title: 'Observado con REA en el tráfico del cliente', className: STATUS_TONE_CLASS.primary },
  'R*': { title: 'Leído con REA en el código del cliente', className: `border-dashed ${STATUS_TONE_CLASS.primary}` },
  C: { title: 'Confirmado por Google o YouTube, papers, el juicio o la filtración', className: STATUS_TONE_CLASS.success },
  I: { title: 'Inferencia de la industria', className: STATUS_TONE_CLASS.warning },
} as const satisfies Record<EvidenceLevel, { title: string; className: string }>;

// Marks wrap in prose so a long note fits a phone; tables scroll sideways, so their marks stay on one line.
const MARK_CLASS = 'evidence-mark mx-0.5 inline-block max-w-full border px-1 align-[0.1em] font-mono text-meta leading-tight';
const HEADING_CLASS = {
  2: 'mt-4 font-display text-section font-bold text-fg-1 uppercase',
  3: 'mt-2 font-display text-title font-bold text-fg-1',
} as const;

export function AlgorithmsGuide(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-8">
      <StudioPageHeader title={PAGE_TITLE} description={PAGE_DESCRIPTION} />

      <nav aria-label="Contenido" className="studio-panel flex flex-col gap-3 p-4">
        <SectionEyebrow label="Contenido" />
        <ol className="grid gap-x-6 gap-y-1.5 text-body-sm @[44rem]/content:grid-cols-2">
          {CONTENTS.map((heading) => (
            <li key={heading.id}>
              <a href={`#${heading.id}`} className="text-fg-2 underline-offset-2 hover:text-primary hover:underline">
                {heading.title}
              </a>
            </li>
          ))}
        </ol>
      </nav>

      <article className="flex flex-col gap-4">
        {BLOCKS.map((block, index) => (
          <GuideBlock key={index} block={block} />
        ))}
      </article>
    </div>
  );
}

function GuideBlock({ block }: { block: Block }): ReactNode {
  switch (block.kind) {
    case 'heading': {
      const Heading = block.level === 2 ? 'h2' : 'h3';
      return (
        <Heading id={block.id} className={`scroll-mt-24 ${HEADING_CLASS[block.level]}`}>
          <InlineContent content={block.content} />
        </Heading>
      );
    }
    case 'paragraph':
      return (
        <p className="measure-read text-body text-fg-2">
          <InlineContent content={block.content} />
        </p>
      );
    case 'list': {
      const items = block.items.map((item, index) => (
        <li key={index} className="pl-1 marker:text-fg-3">
          <InlineContent content={item} />
        </li>
      ));
      const listClass = 'measure-read flex flex-col gap-1.5 pl-5 text-body text-fg-2';
      if (block.start === null) return <ul className={`${listClass} list-disc`}>{items}</ul>;
      return (
        <ol start={block.start} className={`${listClass} list-decimal`}>
          {items}
        </ol>
      );
    }
    case 'table':
      return (
        <div className="overflow-x-auto border border-border">
          <table className="w-full border-collapse text-left text-body-sm [&_.evidence-mark]:whitespace-nowrap">
            <thead className="bg-surface-2">
              <tr>
                {block.head.map((cell, index) => (
                  <th key={index} scope="col" className="px-3 py-2 align-bottom font-mono text-meta font-normal tracking-widest text-fg-3 uppercase">
                    <InlineContent content={cell} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {block.rows.map((row, rowIndex) => (
                <tr key={rowIndex} className="border-t border-border">
                  {row.map((cell, cellIndex) => (
                    <td key={cellIndex} className="min-w-32 px-3 py-2 align-top text-fg-2">
                      <InlineContent content={cell} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    case 'code':
      return (
        <pre className="overflow-x-auto border border-border bg-surface-1 p-4 font-mono text-meta leading-relaxed text-fg-2">
          {block.text}
        </pre>
      );
    case 'rule':
      return <hr className="my-2 border-border" />;
  }
}

function InlineContent({ content }: { content: Inline[] }): ReactNode {
  return content.map((node, index) => <InlineNode key={index} node={node} />);
}

function InlineNode({ node }: { node: Inline }): ReactNode {
  switch (node.kind) {
    case 'text':
      return node.text;
    case 'strong':
      return (
        <strong className="font-semibold text-fg-1">
          <InlineContent content={node.children} />
        </strong>
      );
    case 'em':
      return (
        <em className="text-fg-3">
          <InlineContent content={node.children} />
        </em>
      );
    case 'code':
      return <code className="bg-surface-2 px-1 font-mono text-[0.85em] text-fg-1">{node.text}</code>;
    case 'link':
      return (
        <a href={node.href} target="_blank" rel="noopener noreferrer" className="break-all text-primary underline underline-offset-2">
          {node.href}
        </a>
      );
    case 'evidence': {
      const meta = EVIDENCE_META[node.level];
      return (
        <abbr title={meta.title} className={`${MARK_CLASS} no-underline ${meta.className}`}>
          {node.note === null ? node.level : `${node.level} · ${node.note}`}
        </abbr>
      );
    }
    case 'caveat':
      return <span className={`${MARK_CLASS} ${STATUS_TONE_CLASS.neutral}`}>{node.text}</span>;
  }
}
