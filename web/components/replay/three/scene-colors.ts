/** sRGB triples, 0..1, read off the design tokens so the scene follows the theme. */
export type Rgb = readonly [number, number, number];

export type SceneColors = {
  background: Rgb;
  map: Rgb;
  ct: Rgb;
  t: Rgb;
  bomb: Rgb;
  defuse: Rgb;
  smoke: Rgb;
  flash: Rgb;
};

/**
 * Tokens are oklch; the canvas resolves any CSS colour to sRGB pixels, which
 * Three.js accepts without a colour parser of its own.
 */
function resolver(): (css: string, fallback: Rgb) => Rgb {
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  return (css, fallback) => {
    if (context === null || css === '') return fallback;
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = '#000';
    context.fillStyle = css;
    context.fillRect(0, 0, 1, 1);
    const [r, g, b] = context.getImageData(0, 0, 1, 1).data;
    return [r / 255, g / 255, b / 255];
  };
}

export function readSceneColors(element: HTMLElement): SceneColors {
  const computed = getComputedStyle(element);
  const resolve = resolver();
  const token = (name: string, fallback: Rgb): Rgb => resolve(computed.getPropertyValue(name).trim(), fallback);
  return {
    background: token('--surface-0', [0.02, 0.03, 0.06]),
    map: token('--fg-3', [0.54, 0.6, 0.65]),
    ct: token('--side-ct', [0.42, 0.65, 0.94]),
    t: token('--side-t', [0.95, 0.71, 0.31]),
    bomb: token('--destructive', [1, 0.33, 0.36]),
    defuse: token('--success', [0.36, 0.88, 0.69]),
    smoke: token('--fg-2', [0.75, 0.78, 0.8]),
    flash: token('--fg-1', [0.93, 0.97, 0.98]),
  };
}
