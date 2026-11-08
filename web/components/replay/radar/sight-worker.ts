import { buildSightBvh } from '@/lib/replay/sight-bvh';

/** Builds the sight tree off the main thread and hands its arrays back without a copy. */
addEventListener('message', (event: MessageEvent<unknown>) => {
  const data: unknown = event.data;
  // Always answer, so the map never waits on a worker that ignored its input.
  if (typeof data !== 'object' || data === null || !('vertices' in data) || !('triangles' in data)) return postMessage({ error: 'bad input' });
  const { vertices, triangles } = data;
  if (!(vertices instanceof Float32Array) || !(triangles instanceof Uint32Array)) return postMessage({ error: 'bad input' });
  const bvh = buildSightBvh(vertices, triangles);
  postMessage(bvh, { transfer: [bvh.bounds.buffer, bvh.links.buffer, bvh.triangles.buffer] });
});
