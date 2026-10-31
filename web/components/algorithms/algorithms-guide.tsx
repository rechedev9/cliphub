import type { ReactNode } from 'react';
import guideSource from '@/content/algoritmos.md';
import { parseMarkdown, type Block, type Inline } from '@/lib/algorithms/markdown';
import { StudioPageHeader } from '@/components/studio/page-header';

const PAGE_TITLE = 'Ideas';
const PAGE_DESCRIPTION = 'Qué hacer para que tus Shorts y demos completas lleguen a más gente en YouTube.';

const BLOCKS = parseMarkdown(guideSource);

const HEADING_CLASS = {
  2: 'mt-4 font-display text-section font-bold text-fg-1',
  3: 'mt-2 font-display text-title font-bold text-fg-1',
} as const;

export function AlgorithmsGuide(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-8">
      <StudioPageHeader title={PAGE_TITLE} description={PAGE_DESCRIPTION} />

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
          <table className="w-full border-collapse text-left text-body-sm">
            <thead className="bg-surface-2">
              <tr>
                {block.head.map((cell, index) => (
                  <th key={index} scope="col" className="px-3 py-2 align-bottom text-meta font-medium text-fg-3">
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
        <a href={node.href} target="_blank" rel="noopener noreferrer" className="break-all text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
          {node.href}
        </a>
      );
  }
}
