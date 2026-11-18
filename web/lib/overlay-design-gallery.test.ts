import assert from 'node:assert/strict';
import test from 'node:test';
import { introDesignGallery, introDesignPreviewUrl } from './overlay-design-gallery.ts';

const ARENA = { roster: true, scoreboard: true, theme: 'neon-violet', source: 'demo', mode: 'generated', hud_theme: 'arena' } as const;

test('the gallery offers Neón, chosen by default, and the design that matches the HUD', () => {
  const items = introDesignGallery(ARENA);
  assert.deepEqual(items.map((item) => [item.id, item.label, item.selected]), [['neon-violet', 'Neón', true], ['hud', 'A juego con el HUD', false]]);
  assert.equal(items[0]?.image, '/api/full-demo/overlay-designs/preview?design=neon');
  assert.equal(items[1]?.image, '/api/full-demo/overlay-designs/preview?design=hud&base=arena');
  assert.equal(items[1]?.note, 'Colores de Arena');
  assert.deepEqual(introDesignGallery({ ...ARENA, theme: 'hud' }).map((item) => item.selected), [false, true]);
});

test('the HUD design preview follows the chosen HUD, an own theme and the native HUD', () => {
  const own = { ...ARENA, theme: 'hud', hud_theme: 'prism', hud_custom: { name: 'Mío', ct: '111111', t: '222222', background: '333333', radius: 4 } } as const;
  assert.equal(introDesignPreviewUrl({ design: 'hud', overlays: own, outro: true }), '/api/full-demo/overlay-designs/preview?design=hud&base=prism&ct=111111&t=222222&background=333333&radius=4&outro=1');
  assert.equal(introDesignGallery(own)[1]?.note, 'Colores de Mío');
  const { hud_theme: _theme, ...native } = ARENA;
  assert.equal(introDesignPreviewUrl({ design: 'hud', overlays: native }), '/api/full-demo/overlay-designs/preview?design=hud');
  assert.equal(introDesignGallery(native)[1]?.note, 'Tonos neutros con el HUD de CS2');
});
