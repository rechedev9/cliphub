export function nearestScrollTop({
  itemTop,
  itemBottom,
  scrollTop,
  viewHeight,
}: {
  itemTop: number;
  itemBottom: number;
  scrollTop: number;
  viewHeight: number;
}): number | null {
  if (itemTop < scrollTop) return itemTop;
  if (itemBottom > scrollTop + viewHeight) return Math.min(itemTop, itemBottom - viewHeight);
  return null;
}
