import {
  ClampToEdgeWrapping,
  CompressedArrayTexture,
  CompressedCubeTexture,
  CompressedTexture,
  DataTexture,
  LinearFilter,
  LinearMipmapLinearFilter,
  NoColorSpace,
  RED_GREEN_RGTC2_Format,
  RED_RGTC1_Format,
  RGBAFormat,
  RGBA_BPTC_Format,
  RGBA_S3TC_DXT1_Format,
  RGBA_S3TC_DXT5_Format,
  RGB_BPTC_UNSIGNED_Format,
  RepeatWrapping,
} from 'three';
import type { CompressedPixelFormat, Texture, WebGLRenderer } from 'three';
import type { SceneTexture } from '@/lib/api/map-scene';

type FormatInfo = { format: CompressedPixelFormat; extension: string };

const COMPRESSED: Record<string, FormatInfo> = {
  bc1: { format: RGBA_S3TC_DXT1_Format, extension: 'WEBGL_compressed_texture_s3tc' },
  bc3: { format: RGBA_S3TC_DXT5_Format, extension: 'WEBGL_compressed_texture_s3tc' },
  bc4: { format: RED_RGTC1_Format, extension: 'EXT_texture_compression_rgtc' },
  bc5: { format: RED_GREEN_RGTC2_Format, extension: 'EXT_texture_compression_rgtc' },
  bc6h: { format: RGB_BPTC_UNSIGNED_Format, extension: 'EXT_texture_compression_bptc' },
  bc7: { format: RGBA_BPTC_Format, extension: 'EXT_texture_compression_bptc' },
};

export function toTexture(source: SceneTexture, renderer: WebGLRenderer, wrap: boolean): Texture | null {
  if (source.faces !== 1) return null;
  if (source.format === 'rgba8' || source.format === 'bgra8') {
    const data = new Uint8Array(source.levels[0]);
    if (source.format === 'bgra8') {
      for (let i = 0; i + 3 < data.length; i += 4) {
        const b = data[i];
        data[i] = data[i + 2];
        data[i + 2] = b;
      }
    }
    const texture = new DataTexture(data, source.width, source.height, RGBAFormat);
    texture.generateMipmaps = true;
    texture.minFilter = LinearMipmapLinearFilter;
    texture.needsUpdate = true;
    return texture;
  }
  const info = COMPRESSED[source.format];
  if (info === undefined || !renderer.extensions.has(info.extension)) return null;
  // WebGL wants every level of a compressed chain; a 1 x 1 level still holds one 4 x 4 block.
  const mipmaps = source.levels.map((data, level) => ({ data, width: Math.max(1, source.width >> level), height: Math.max(1, source.height >> level) }));
  const texture = new CompressedTexture(mipmaps, source.width, source.height, info.format);
  texture.colorSpace = NoColorSpace;
  texture.minFilter = mipmaps.length > 1 ? LinearMipmapLinearFilter : LinearFilter;
  texture.magFilter = LinearFilter;
  texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy());
  if (wrap) {
    texture.wrapS = RepeatWrapping;
    texture.wrapT = RepeatWrapping;
  }
  texture.needsUpdate = true;
  return texture;
}

/** A cube array as a 2D array of its faces, 6 layers per cube; null when the GPU cannot read its format. */
export function toCubeArrayTexture(source: SceneTexture, renderer: WebGLRenderer): CompressedArrayTexture | null {
  const info = COMPRESSED[source.format];
  if (source.faces < 6 || source.faces % 6 !== 0 || info === undefined || !renderer.extensions.has(info.extension)) return null;
  const mipmaps = source.levels.map((data, level) => ({ data, width: Math.max(1, source.width >> level), height: Math.max(1, source.height >> level) }));
  const texture = new CompressedArrayTexture(mipmaps, source.width, source.height, source.faces, info.format);
  texture.colorSpace = NoColorSpace;
  texture.minFilter = mipmaps.length > 1 ? LinearMipmapLinearFilter : LinearFilter;
  texture.magFilter = LinearFilter;
  texture.wrapS = ClampToEdgeWrapping;
  texture.wrapT = ClampToEdgeWrapping;
  texture.generateMipmaps = false;
  texture.needsUpdate = true;
  return texture;
}

/** A sky cube from the scene, its faces split out of every level; null when the GPU cannot read its format. */
export function toCubeTexture(source: SceneTexture, renderer: WebGLRenderer): CompressedCubeTexture | null {
  const info = COMPRESSED[source.format];
  if (source.faces !== 6 || info === undefined || !renderer.extensions.has(info.extension)) return null;
  const faces: CompressedTexture[] = [];
  for (let face = 0; face < 6; face++) {
    const mipmaps = source.levels.map((data, level) => {
      const size = data.length / 6;
      return { data: data.subarray(face * size, (face + 1) * size), width: Math.max(1, source.width >> level), height: Math.max(1, source.height >> level) };
    });
    faces.push(new CompressedTexture(mipmaps, source.width, source.height, info.format));
  }
  const cube = new CompressedCubeTexture(faces, info.format);
  cube.colorSpace = NoColorSpace;
  cube.minFilter = source.levels.length > 1 ? LinearMipmapLinearFilter : LinearFilter;
  cube.magFilter = LinearFilter;
  cube.generateMipmaps = false;
  cube.needsUpdate = true;
  return cube;
}
