import test from 'node:test';
import assert from 'node:assert/strict';
import {
  SHARE_FILE,
  packageSizeLabel,
  reportShareSummary,
  shareErrorLabel,
  shareImportSummary,
  type SharePackageImport,
} from './share.ts';

const imported: SharePackageImport = {
  kind: 'report', title: 'Prueba Mirage', notes: '', duplicate: false, matches_imported: 2, matches_existing: 1, annotations: 1,
};

test('package sizes read like the file explorer', () => {
  assert.equal(packageSizeLabel(300), '1 KB');
  assert.equal(packageSizeLabel(512 * 1024), '512 KB');
  assert.equal(packageSizeLabel(3.25 * 1024 * 1024), '3,3 MB');
  assert.equal(packageSizeLabel(42.4 * 1024 * 1024), '42 MB');
});

test('the share summary counts matches', () => {
  assert.equal(reportShareSummary(3), 'Lleva el informe y 3 partidas con sus rondas, posiciones y notas, sin las demos.');
});

test('the import summary names what was added', () => {
  assert.equal(
    shareImportSummary(imported),
    'El informe «Prueba Mirage» está en tu biblioteca: 2 partidas nuevas, 1 partida que ya tenías, 1 nota.',
  );
  assert.equal(
    shareImportSummary({ ...imported, matches_imported: 0, matches_existing: 0, annotations: 0 }),
    'El informe «Prueba Mirage» está en tu biblioteca.',
  );
  assert.equal(shareImportSummary({ ...imported, duplicate: true }), 'El informe «Prueba Mirage» ya estaba en tu biblioteca.');
});

test('known error codes get their own sentence', () => {
  assert.match(shareErrorLabel('share_package_invalid', 'x'), /no es un paquete/);
  assert.equal(shareErrorLabel('other', 'Fallo.'), 'Fallo.');
  assert.equal(shareErrorLabel(undefined, 'Fallo.'), 'Fallo.');
});

test('only .cliphub files are packages', () => {
  assert.ok(SHARE_FILE.test('Prueba.CLIPHUB'));
  assert.ok(!SHARE_FILE.test('match.dem'));
});
