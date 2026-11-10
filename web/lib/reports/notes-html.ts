import { parseMarkdown, type Block, type Inline } from '../algorithms/markdown.ts';

/** HTML-escapes text; the export never lets the notes write markup of their own. */
function escapeText(value: string): string {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&#39;');
}

function inlineHtml(content: readonly Inline[], origin: string): string {
  return content
    .map((node) => {
      switch (node.kind) {
        case 'text':
          return escapeText(node.text);
        case 'code':
          return `<code>${escapeText(node.text)}</code>`;
        case 'strong':
          return `<strong>${inlineHtml(node.children, origin)}</strong>`;
        case 'em':
          return `<em>${inlineHtml(node.children, origin)}</em>`;
        case 'route': {
          const label = inlineHtml(node.children, origin);
          return origin === '' ? label : `<a href="${escapeText(origin + node.href)}">${label}</a>`;
        }
        case 'link':
          // The export links only into the Studio that made it; outside addresses stay text.
          return escapeText(node.href);
      }
    })
    .join('');
}

function blockHtml(block: Block, origin: string): string {
  switch (block.kind) {
    case 'heading':
      return block.level === 2 ? `<h3>${inlineHtml(block.content, origin)}</h3>` : `<h4>${inlineHtml(block.content, origin)}</h4>`;
    case 'paragraph':
      return `<p>${inlineHtml(block.content, origin)}</p>`;
    case 'list': {
      const items = block.items.map((item) => `<li>${inlineHtml(item, origin)}</li>`).join('');
      return block.start === null ? `<ul>${items}</ul>` : `<ol start="${block.start}">${items}</ol>`;
    }
    case 'table': {
      const head = block.head.map((cell) => `<th>${inlineHtml(cell, origin)}</th>`).join('');
      const rows = block.rows.map((row) => `<tr>${row.map((cell) => `<td>${inlineHtml(cell, origin)}</td>`).join('')}</tr>`).join('');
      return `<table><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table>`;
    }
    case 'code':
      return `<pre>${escapeText(block.text)}</pre>`;
    case 'rule':
      return '<hr>';
  }
}

/** The report notes as HTML for the export: the same Markdown the Notas tab renders, Studio links on `origin`. */
export function notesHtml(source: string, origin: string): string {
  return parseMarkdown(source).map((block) => blockHtml(block, origin)).join('\n');
}
