import test from 'node:test';
import assert from 'node:assert/strict';
import { defaultTeamDocumentTitle, formatDocumentSize, teamDocumentFileProblem, TEAM_DOCUMENT_POLICY, TEAM_DOCUMENT_SANDBOX } from './team-documents.ts';

test('only an HTML file of up to 5 MB can be picked', () => {
  assert.equal(teamDocumentFileProblem({ name: 'Antistrats SEGA.HTML', size: 2048, type: '' }), null);
  assert.equal(teamDocumentFileProblem({ name: 'informe', size: 2048, type: 'text/html' }), null);
  assert.match(teamDocumentFileProblem({ name: 'informe.pdf', size: 2048, type: 'application/pdf' }) ?? '', /HTML/);
  assert.match(teamDocumentFileProblem({ name: 'a.htm', size: 0, type: '' }) ?? '', /vacío/);
  assert.match(teamDocumentFileProblem({ name: 'a.html', size: 5 * 1024 * 1024 + 1, type: '' }) ?? '', /5 MB/);
});

test('a document is titled by its file name and sized for people', () => {
  assert.equal(defaultTeamDocumentTitle('sega-cache-pistolas.html'), 'sega-cache-pistolas');
  assert.equal(defaultTeamDocumentTitle(' Vetos BO3.HTM'), 'Vetos BO3');
  assert.equal(formatDocumentSize(900), '900 B');
  assert.equal(formatDocumentSize(40 * 1024), '40 KB');
  assert.equal(formatDocumentSize(1.5 * 1024 * 1024), '1,5 MB');
});

test('a document never shares the app origin', () => {
  assert.ok(!TEAM_DOCUMENT_POLICY.includes('allow-same-origin'));
  assert.ok(!TEAM_DOCUMENT_SANDBOX.includes('allow-same-origin'));
  assert.match(TEAM_DOCUMENT_POLICY, /^sandbox allow-scripts /);
  assert.match(TEAM_DOCUMENT_POLICY, /connect-src 'none'/);
});
