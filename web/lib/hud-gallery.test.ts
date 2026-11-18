import assert from 'node:assert/strict';
import test from 'node:test';
import { fullDemoHudGallery, verticalHudGallery } from './hud-gallery.ts';

const MINE = { id: '0b6e2f4a-1c3d-4e5f-8a9b-0c1d2e3f4a5b', base: 'prism', name: 'Mío', ct: '111111', t: '222222', background: '333333' };

test('the Full Demo gallery shows the catalog in its order, then the own themes, each with its picture', () => {
  const { items } = fullDemoHudGallery({ base: 'arena', themes: [MINE] });
  assert.deepEqual(items.map((item) => item.label), ['Arena', 'Apex', 'Prism', 'Carbon', 'Pulse', 'Royale', 'Glacier', 'Ember', 'Circuit', 'Focus', 'Mono', 'Mío']);
  assert.deepEqual(items[0], { id: 'arena', label: 'Arena', image: '/hud/arena.webp', selected: true });
  assert.deepEqual(items.at(-1), {
    id: 'user:0b6e2f4a-1c3d-4e5f-8a9b-0c1d2e3f4a5b', label: 'Mío', note: 'Tema propio', selected: false,
    image: '/api/full-demo/hud-themes/preview?base=prism&ct=111111&t=222222&background=333333',
  });
  assert.deepEqual(items.filter((item) => item.selected).map((item) => item.id), ['arena']);
});

test('a plan with a saved theme marks that theme, and keeps it marked while its colours are edited', () => {
  const chosen = (gallery: { items: { id: string; selected: boolean }[] }): string[] => gallery.items.filter((item) => item.selected).map((item) => item.id);
  const saved = fullDemoHudGallery({ base: 'prism', style: { name: 'Mío', ct: '111111', t: '222222', background: '333333' }, themes: [MINE] });
  assert.deepEqual(chosen(saved), ['user:0b6e2f4a-1c3d-4e5f-8a9b-0c1d2e3f4a5b']);
  assert.equal(saved.editing?.name, 'Mío');
  const edited = { name: 'Mío', ct: 'ABCDEF', t: '222222', background: '333333' };
  const editing = fullDemoHudGallery({ base: 'prism', style: edited, themes: [MINE], editingId: MINE.id });
  assert.deepEqual(chosen(editing), ['user:0b6e2f4a-1c3d-4e5f-8a9b-0c1d2e3f4a5b']);
  const gone = fullDemoHudGallery({ base: 'prism', style: edited, themes: [MINE] });
  assert.deepEqual(chosen(gone), ['prism']);
  assert.equal(gone.editing, undefined);
});

test('the Short scoreboard gallery shows the vertical band of each catalog design and marks the first by default', () => {
  const items = verticalHudGallery('glacier');
  assert.deepEqual(items.filter((item) => item.selected).map((item) => item.id), ['glacier']);
  assert.deepEqual(items.find((item) => item.selected), { id: 'glacier', label: 'Glacier', image: '/api/full-demo/hud-themes/preview?base=glacier&vertical=1', selected: true });
  assert.deepEqual(verticalHudGallery(undefined).filter((item) => item.selected).map((item) => item.id), ['arena']);
});
