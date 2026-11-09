// gifenc 1.0.3 ships no types; this covers the calls the Replay's GIF export makes.
declare module 'gifenc' {
  export type GifColorFormat = 'rgb565' | 'rgb444' | 'rgba4444';
  export type GifPalette = number[][];

  export type GifFrameOptions = {
    palette?: GifPalette;
    delay?: number;
    repeat?: number;
    transparent?: boolean;
    transparentIndex?: number;
    dispose?: number;
  };

  export type GifEncoderStream = {
    writeFrame(index: Uint8Array, width: number, height: number, options?: GifFrameOptions): void;
    finish(): void;
    bytes(): Uint8Array<ArrayBuffer>;
  };

  export function GIFEncoder(options?: { initialCapacity?: number; auto?: boolean }): GifEncoderStream;
  export function quantize(rgba: Uint8Array | Uint8ClampedArray, maxColors: number, options?: { format?: GifColorFormat }): GifPalette;
  export function applyPalette(rgba: Uint8Array | Uint8ClampedArray, palette: GifPalette, format?: GifColorFormat): Uint8Array;
}
