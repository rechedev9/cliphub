/** Conditional GET helpers for hub list polls. */

export type ConditionalJSONCache<T> = {
  etag?: string;
  value: T;
};

export function ifNoneMatchInit(etag: string | undefined): RequestInit {
  return {
    cache: 'no-store',
    headers: etag ? { 'If-None-Match': etag } : {},
  };
}

export async function readConditionalJSON<T>(
  res: Response,
  cache: ConditionalJSONCache<T> | null,
  parse: (res: Response) => Promise<T>,
): Promise<ConditionalJSONCache<T>> {
  if (res.status === 304) {
    if (cache === null) throw new Error('list revalidation returned 304 without a cached body');
    return { value: cache.value, etag: res.headers.get('ETag') ?? cache.etag };
  }
  return { value: await parse(res), etag: res.headers.get('ETag') ?? undefined };
}
