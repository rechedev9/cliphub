/**
 * Hand-off for demo files chosen on the hub's empty state. `File` objects cannot
 * ride a URL, so the hub parks them here and `/clips/nueva` takes them on mount.
 */
let pending: File[] = [];

export function setPendingDemoFiles(files: readonly File[]): void {
  pending = [...files];
}

/** Returns the parked files once; a second call gets an empty list. */
export function takePendingDemoFiles(): File[] {
  const files = pending;
  pending = [];
  return files;
}
