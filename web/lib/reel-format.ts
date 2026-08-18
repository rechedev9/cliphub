import type { RenderFormat } from './api/types.ts';

export const REEL_FORMAT_ITEMS: Array<{ value: RenderFormat; label: string }> = [
  { value: 'short-9x16', label: '9:16' },
  { value: 'landscape-16x9', label: '16:9' },
];

export function lockedFormatLabel(format: RenderFormat): string {
  return REEL_FORMAT_ITEMS.find((item) => item.value === format)?.label ?? '16:9';
}
