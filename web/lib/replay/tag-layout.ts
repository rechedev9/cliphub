/** A screen rectangle in CSS pixels, top-left anchored. */
export type TagRect = { x: number; y: number; width: number; height: number };

/** One name to place next to its player's disc. */
export type TagRequest = { id: number; x: number; y: number; width: number; height: number };

/** `spot` is the candidate used, to pass back as `prefer` next frame. */
export type PlacedTag = { id: number; rect: TagRect; spot: number; displaced: boolean };

function overlaps(a: TagRect, b: TagRect): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

function clampInto(rect: TagRect, bounds: TagRect): TagRect {
  return {
    ...rect,
    x: Math.max(bounds.x, Math.min(bounds.x + bounds.width - rect.width, rect.x)),
    y: Math.max(bounds.y, Math.min(bounds.y + bounds.height - rect.height, rect.y)),
  };
}

const RING_SPOTS = 8;
const RINGS = 4;

function candidate(tag: TagRequest, gap: number, index: number): TagRect {
  const { width, height } = tag;
  const far = gap + Math.floor(index / RING_SPOTS) * (height + 2);
  const d = far * 0.7;
  switch (index % RING_SPOTS) {
    case 0:
      return { x: tag.x + far, y: tag.y - height / 2, width, height };
    case 1:
      return { x: tag.x - far - width, y: tag.y - height / 2, width, height };
    case 2:
      return { x: tag.x - width / 2, y: tag.y - far - height, width, height };
    case 3:
      return { x: tag.x - width / 2, y: tag.y + far, width, height };
    case 4:
      return { x: tag.x + d, y: tag.y - d - height, width, height };
    case 5:
      return { x: tag.x + d, y: tag.y + d, width, height };
    case 6:
      return { x: tag.x - d - width, y: tag.y - d - height, width, height };
    default:
      return { x: tag.x - d - width, y: tag.y + d, width, height };
  }
}

/**
 * Where each name goes: beside its disc on the right first, then left, above,
 * below and the diagonals, then the same ring further out. A spot must clear
 * every tag already placed and every disc, and stay inside `bounds`. A tag
 * keeps last frame's spot (`prefer`) while it is free, so names do not jump.
 * When no spot is free the first candidate wins, so a tag is never dropped.
 */
export function layoutTags(options: {
  tags: readonly TagRequest[];
  discs: readonly { x: number; y: number }[];
  radius: number;
  bounds: TagRect;
  prefer?: ReadonlyMap<number, number>;
}): PlacedTag[] {
  const { tags, discs, radius, bounds, prefer } = options;
  const gap = radius + 4;
  const blockers: TagRect[] = discs.map((disc) => ({ x: disc.x - radius, y: disc.y - radius, width: radius * 2, height: radius * 2 }));
  const placed: PlacedTag[] = [];
  const taken: TagRect[] = [];
  const free = (spot: TagRect): boolean => !taken.some((other) => overlaps(other, spot)) && !blockers.some((other) => overlaps(other, spot));
  for (const tag of tags) {
    const preferred = prefer?.get(tag.id);
    const order = Array.from({ length: RING_SPOTS * RINGS }, (_, index) => index);
    if (preferred !== undefined && preferred < order.length) order.unshift(preferred);
    let chosen: { rect: TagRect; index: number } | null = null;
    for (const index of order) {
      const spot = clampInto(candidate(tag, gap, index), bounds);
      if (free(spot)) {
        chosen = { rect: spot, index };
        break;
      }
    }
    const rect = chosen?.rect ?? clampInto(candidate(tag, gap, 0), bounds);
    taken.push(rect);
    placed.push({ id: tag.id, rect, spot: chosen?.index ?? 0, displaced: chosen === null || chosen.index >= RING_SPOTS });
  }
  return placed;
}

/** A name shortened to `max` characters with an ellipsis; never cut to a stub. */
export function tagName(name: string, max: number): string {
  const trimmed = name.trim();
  if (trimmed.length <= max) return trimmed;
  return `${trimmed.slice(0, Math.max(1, max - 1)).trimEnd()}…`;
}
