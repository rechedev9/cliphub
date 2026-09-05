/** Only completed downloads initiated by Studio may be revealed. Renderer input is never a path. */
export class StreamDownloads {
  private readonly paths = new Map<string, string>();
  key(value: unknown, origin: string | null): string | null {
    if (typeof value !== 'string' || value.length > 4096 || !origin) return null;
    try {
      const url = new URL(value);
      if (url.origin !== origin || url.username || url.password || url.hash) return null;
      if (!/^\/api\/streams\/[0-9a-f-]{36}\/renders\/[a-zA-Z0-9_-]+\/videos\/[a-zA-Z0-9._-]+$/.test(url.pathname))
        return null;
      return url.href;
    } catch {
      return null;
    }
  }
  completed(url: string, path: string): void {
    this.paths.delete(url);
    this.paths.set(url, path);
    if (this.paths.size > 100) this.paths.delete(this.paths.keys().next().value!);
  }
  savedPath(url: string): string | undefined {
    return this.paths.get(url);
  }
}
