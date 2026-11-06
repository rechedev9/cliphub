import { CanvasTexture, SRGBColorSpace } from 'three';
import { seededRandom } from '@/lib/replay/three/utility-visuals';

/**
 * Small textures the grenades are drawn with, painted once per scene on a
 * canvas: a soft noisy puff for smoke, a flame tongue, a burnt floor and a glow.
 */

function paint(width: number, height: number, draw: (context: CanvasRenderingContext2D) => void): CanvasTexture {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d');
  if (context !== null) draw(context);
  const texture = new CanvasTexture(canvas);
  texture.colorSpace = SRGBColorSpace;
  return texture;
}

/** A white cloud blob: many soft circles inside a round falloff, so overlapping puffs read as volume. */
function puff(): CanvasTexture {
  return paint(128, 128, (context) => {
    const random = seededRandom(11);
    for (let i = 0; i < 46; i += 1) {
      const angle = random() * Math.PI * 2;
      const reach = Math.sqrt(random()) * 34;
      const x = 64 + Math.cos(angle) * reach;
      const y = 64 + Math.sin(angle) * reach;
      const radius = 14 + random() * 22;
      const gradient = context.createRadialGradient(x, y, 0, x, y, radius);
      const shade = Math.round(215 + random() * 40);
      gradient.addColorStop(0, `rgba(${shade}, ${shade}, ${shade}, 0.32)`);
      gradient.addColorStop(1, `rgba(${shade}, ${shade}, ${shade}, 0)`);
      context.fillStyle = gradient;
      context.fillRect(0, 0, 128, 128);
    }
    context.globalCompositeOperation = 'destination-in';
    const mask = context.createRadialGradient(64, 64, 18, 64, 64, 63);
    mask.addColorStop(0, 'rgba(0, 0, 0, 1)');
    mask.addColorStop(1, 'rgba(0, 0, 0, 0)');
    context.fillStyle = mask;
    context.fillRect(0, 0, 128, 128);
  });
}

/** One flame tongue, white-hot at its foot and red at its tip, anchored at the bottom edge. */
function flame(): CanvasTexture {
  return paint(64, 128, (context) => {
    const gradient = context.createLinearGradient(0, 128, 0, 0);
    gradient.addColorStop(0, 'rgba(255, 244, 200, 1)');
    gradient.addColorStop(0.25, 'rgba(255, 186, 70, 0.95)');
    gradient.addColorStop(0.6, 'rgba(240, 90, 20, 0.6)');
    gradient.addColorStop(1, 'rgba(160, 30, 10, 0)');
    context.fillStyle = gradient;
    context.beginPath();
    context.moveTo(32, 2);
    context.bezierCurveTo(46, 40, 62, 72, 56, 104);
    context.bezierCurveTo(52, 124, 12, 124, 8, 104);
    context.bezierCurveTo(2, 72, 18, 40, 32, 2);
    context.fill();
  });
}

/** The burning floor under a molotov: a hot core, scattered embers and a ragged edge. */
function fireFloor(): CanvasTexture {
  return paint(128, 128, (context) => {
    const base = context.createRadialGradient(64, 64, 4, 64, 64, 62);
    base.addColorStop(0, 'rgba(255, 170, 60, 0.95)');
    base.addColorStop(0.55, 'rgba(230, 80, 20, 0.7)');
    base.addColorStop(1, 'rgba(120, 20, 5, 0)');
    context.fillStyle = base;
    context.fillRect(0, 0, 128, 128);
    const random = seededRandom(5);
    for (let i = 0; i < 40; i += 1) {
      const angle = random() * Math.PI * 2;
      const reach = Math.sqrt(random()) * 54;
      const x = 64 + Math.cos(angle) * reach;
      const y = 64 + Math.sin(angle) * reach;
      const radius = 4 + random() * 10;
      const ember = context.createRadialGradient(x, y, 0, x, y, radius);
      ember.addColorStop(0, 'rgba(255, 225, 140, 0.8)');
      ember.addColorStop(1, 'rgba(255, 120, 30, 0)');
      context.fillStyle = ember;
      context.fillRect(0, 0, 128, 128);
    }
  });
}

/** A round glow, white in the middle and gone at the edge; tinted by the material. */
function glow(): CanvasTexture {
  return paint(64, 64, (context) => {
    const gradient = context.createRadialGradient(32, 32, 0, 32, 32, 32);
    gradient.addColorStop(0, 'rgba(255, 255, 255, 1)');
    gradient.addColorStop(0.25, 'rgba(255, 255, 255, 0.55)');
    gradient.addColorStop(1, 'rgba(255, 255, 255, 0)');
    context.fillStyle = gradient;
    context.fillRect(0, 0, 64, 64);
  });
}

/** A filled dot with a dark rim, the grenade in flight until its icon loads. */
function dot(): CanvasTexture {
  return paint(32, 32, (context) => {
    context.fillStyle = 'rgba(10, 12, 14, 0.85)';
    context.beginPath();
    context.arc(16, 16, 15, 0, Math.PI * 2);
    context.fill();
    context.fillStyle = '#fff';
    context.beginPath();
    context.arc(16, 16, 10, 0, Math.PI * 2);
    context.fill();
  });
}

export type UtilityTextures = {
  puff: CanvasTexture;
  flame: CanvasTexture;
  fireFloor: CanvasTexture;
  glow: CanvasTexture;
  dot: CanvasTexture;
};

export function createUtilityTextures(): UtilityTextures {
  return { puff: puff(), flame: flame(), fireFloor: fireFloor(), glow: glow(), dot: dot() };
}

export function disposeUtilityTextures(textures: UtilityTextures): void {
  for (const texture of Object.values(textures)) texture.dispose();
}
