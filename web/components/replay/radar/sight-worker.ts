import { buildSightBvh } from '@/lib/replay/sight-bvh';

/** Builds the sight tree off the main thread and hands its arrays back without a copy. */
addEventListener('message', (event: MessageEvent<unknown>) => {
  const data: unknown = event.data;
  if (typeof data !== 'object' || data === null || !('vertices' in data) || !('triangles' in data)) return;
  const { vertices, triangles } = data;
  if (!(vertices instanceof Float32Array) || !(triangles instanceof Uint32Array)) return;
  const bvh = buildSightBvh(vertices, triangles);
  postMessage(bvh, { transfer: [bvh.bounds.buffer, bvh.links.buffer, bvh.triangles.buffer] });
});
