import type { MetricsReport } from '../api/metrics.ts';
import type { ReportSide, ScoutMatchSummary, ScoutReport } from '../api/scout.ts';
import { metricsMatchIndex, metricsSections, modelText } from '../metrics/advanced.ts';
import {
  REPORT_SIDE_LABEL,
  evidenceLabel,
  evidencePath,
  evidenceTitle,
  matchIndex,
  scoutSections,
  thresholdsText,
  type ReportCell,
  type ReportSection,
  type ReportTable,
} from './scout-sections.ts';

/**
 * The scouting report as one self-contained HTML file: inline styles, no
 * script, no external resource, laid out to print on A4. Links point at the
 * Studio that exported it, so they open while Studio runs on that PC.
 */

export type ReportExportInput = {
  readonly title: string;
  readonly teamName: string;
  readonly side: ReportSide;
  readonly notes: string;
  readonly scout: ScoutReport;
  /** The players' advanced metrics, appended after the scouting sections when present. */
  readonly metrics?: MetricsReport | null;
  /** Origin of the exporting Studio, such as http://127.0.0.1:3000; empty leaves plain text. */
  readonly origin: string;
  readonly generatedAt: Date;
};

export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

const STYLE = `
:root { color-scheme: light; }
* { box-sizing: border-box; }
body { margin: 0; padding: 32px; font: 13px/1.45 "Segoe UI", system-ui, sans-serif; color: #18181b; background: #ffffff; font-variant-numeric: tabular-nums; }
header { border-bottom: 2px solid #18181b; padding-bottom: 12px; margin-bottom: 20px; }
.kind { margin: 0; font-size: 11px; letter-spacing: .08em; text-transform: uppercase; color: #52525b; }
h1 { margin: 4px 0 6px; font-size: 24px; }
h2 { margin: 28px 0 4px; font-size: 17px; border-bottom: 1px solid #d4d4d8; padding-bottom: 4px; }
h3 { margin: 16px 0 6px; font-size: 13px; }
.meta, .intro, footer { color: #52525b; }
.intro { margin: 0 0 8px; }
.notes { white-space: pre-wrap; border-left: 3px solid #0891b2; padding: 8px 12px; background: #f4f4f5; margin: 0 0 16px; }
dl { display: flex; flex-wrap: wrap; gap: 8px 24px; margin: 8px 0; }
dt { font-size: 11px; color: #52525b; }
dd { margin: 0; font-weight: 600; }
table { width: 100%; border-collapse: collapse; margin: 0 0 8px; }
th, td { text-align: left; padding: 4px 6px; border-bottom: 1px solid #e4e4e7; vertical-align: top; }
th { font-size: 11px; color: #52525b; font-weight: 600; background: #f4f4f5; }
td.num { text-align: right; white-space: nowrap; }
.low { color: #71717a; font-style: italic; }
.win { color: #15803d; } .loss { color: #b91c1c; } .ct { color: #2563eb; } .t { color: #b45309; }
a { color: #0e7490; }
.ev { display: inline-block; margin: 0 4px 2px 0; padding: 0 4px; border: 1px solid #d4d4d8; border-radius: 3px; font-size: 11px; white-space: nowrap; text-decoration: none; }
.empty { color: #71717a; font-style: italic; margin: 0 0 8px; }
footer { margin-top: 32px; border-top: 1px solid #d4d4d8; padding-top: 8px; font-size: 11px; }
section { break-inside: auto; }
tr, h2, h3 { break-inside: avoid; break-after: avoid-page; }
@page { size: A4; margin: 14mm; }
@media print { body { padding: 0; } a { color: inherit; } }
`;

function link(href: string, label: string, origin: string, attrs = ''): string {
  if (origin === '') return `<span${attrs}>${escapeHtml(label)}</span>`;
  return `<a href="${escapeHtml(origin + href)}"${attrs}>${escapeHtml(label)}</a>`;
}

function cellHtml(cell: ReportCell, origin: string, matches: ReadonlyMap<string, ScoutMatchSummary>): string {
  const classes = [cell.numeric ? 'num' : '', cell.lowSample ? 'low' : '', cell.tone ?? ''].filter((c) => c !== '');
  const classAttr = classes.length > 0 ? ` class="${classes.join(' ')}"` : '';
  const title = cell.lowSample ? ' title="Muestra pequeña"' : '';
  let body = cell.href ? link(cell.href, cell.text, origin) : escapeHtml(cell.text);
  if (cell.evidence && cell.evidence.length > 0) {
    body = cell.evidence
      .map((evidence) =>
        link(evidencePath(evidence), evidenceLabel(evidence, matches), origin, ` class="ev" title="${escapeHtml(evidenceTitle(evidence, matches))}"`),
      )
      .join('');
  }
  return `<td${classAttr}${title}>${body}</td>`;
}

function tableHtml(table: ReportTable, origin: string, matches: ReadonlyMap<string, ScoutMatchSummary>): string {
  const heading = `<h3>${escapeHtml(table.caption)}</h3>`;
  if (table.rows.length === 0) return `${heading}<p class="empty">${escapeHtml(table.empty)}</p>`;
  const head = table.columns.map((column) => `<th scope="col">${escapeHtml(column)}</th>`).join('');
  const rows = table.rows.map((row) => `<tr>${row.map((cell) => cellHtml(cell, origin, matches)).join('')}</tr>`).join('\n');
  return `${heading}<table><thead><tr>${head}</tr></thead><tbody>\n${rows}\n</tbody></table>`;
}

function sectionHtml(section: ReportSection, origin: string, matches: ReadonlyMap<string, ScoutMatchSummary>): string {
  const facts =
    section.facts.length === 0
      ? ''
      : `<dl>${section.facts.map((fact) => `<div><dt>${escapeHtml(fact.label)}</dt><dd>${escapeHtml(fact.value)}</dd></div>`).join('')}</dl>`;
  const tables = section.tables.map((table) => tableHtml(table, origin, matches)).join('\n');
  return `<section id="${escapeHtml(section.id)}"><h2>${escapeHtml(section.title)}</h2><p class="intro">${escapeHtml(section.intro)}</p>${facts}\n${tables}</section>`;
}

const generatedFormat = new Intl.DateTimeFormat('es-ES', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  timeZone: 'Europe/Madrid',
});

/** The whole report as a standalone, printable HTML document. */
export function buildReportHtml(input: ReportExportInput): string {
  const { scout, origin } = input;
  const matches = matchIndex(scout);
  const maps = [...new Set(scout.matches.map((match) => match.map))].length;
  const meta = [
    `Rival: ${input.teamName}`,
    `Lado: ${REPORT_SIDE_LABEL[input.side]}`,
    `${scout.matches.length} ${scout.matches.length === 1 ? 'partida' : 'partidas'} en ${maps} ${maps === 1 ? 'mapa' : 'mapas'}`,
    `Generado el ${generatedFormat.format(input.generatedAt)}`,
  ].join('. ');
  const notes = input.notes.trim() === '' ? '' : `<p class="notes">${escapeHtml(input.notes.trim())}</p>`;
  const metricsMatches = input.metrics ? metricsMatchIndex(input.metrics, matches) : matches;
  const sections = [
    ...scoutSections(scout).map((section) => sectionHtml(section, origin, matches)),
    ...(input.metrics ? metricsSections(input.metrics).map((section) => sectionHtml(section, origin, metricsMatches)) : []),
  ].join('\n');
  const model = input.metrics ? `<p>${escapeHtml(modelText(input.metrics))}</p>` : '';
  const linkNote =
    origin === ''
      ? 'Las rondas de ejemplo se citan por mapa y número de ronda.'
      : 'Cada ronda de ejemplo enlaza a su partida, su ronda y su tick en ClipHub Studio, y abre mientras Studio está abierto en el PC que exportó el informe.';
  return `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escapeHtml(input.title)}</title>
<style>${STYLE}</style>
</head>
<body>
<header><p class="kind">Informe de scouting</p><h1>${escapeHtml(input.title)}</h1><p class="meta">${escapeHtml(meta)}.</p></header>
${notes}
${sections}
<footer><p>${escapeHtml(thresholdsText(scout))}</p>${model}<p>${escapeHtml(linkNote)}</p><p>Generado con ClipHub Studio a partir de las demos de este PC.</p></footer>
</body>
</html>
`;
}

/** A download name for the report: "informe-vitality-mirage.html". */
export function reportFileStem(title: string): string {
  const slug = title
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60)
    .replace(/-+$/g, '');
  return `informe-${slug === '' ? 'scouting' : slug}`;
}
