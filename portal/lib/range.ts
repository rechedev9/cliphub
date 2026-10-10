export interface ByteRange {
  start: number;
  // Inclusive.
  end: number;
}

// Parses a single "bytes=N-" or "bytes=N-M" header against a file of the given size.
// null means no header (send the whole file); "invalid" means answer 416.
export function parseRange(
  header: string | null,
  size: number,
): ByteRange | "invalid" | null {
  if (header === null) return null;
  const match = /^bytes=(\d+)-(\d*)$/.exec(header.trim());
  if (!match) return "invalid";
  const start = Number(match[1]);
  const end = match[2] ? Number(match[2]) : size - 1;
  if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end)) return "invalid";
  if (start >= size || end < start) return "invalid";
  return { start, end: Math.min(end, size - 1) };
}
