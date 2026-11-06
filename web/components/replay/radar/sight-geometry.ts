import { buildSightBvh } from '@/lib/replay/sight-bvh';
import type { SightBvh } from '@/lib/replay/sight-bvh';
import { loadMapGeometry } from '../three/geometry-cache';

export type SightGeometry = { kind: 'ready'; bvh: SightBvh } | { kind: 'failed'; message: string };

const NO_WALLS = 'No se pudieron leer las paredes del mapa desde tu instalación de CS2, así que no se sabe quién ve a quién.';

/** One tree per map for the session; a failed read is forgotten so turning the layer on again retries. */
const cache = new Map<string, Promise<SightGeometry>>();

function asBvh(data: unknown): SightBvh | null {
  if (typeof data !== 'object' || data === null) return null;
  if (!('bounds' in data) || !('links' in data) || !('triangles' in data) || !('nodeCount' in data)) return null;
  const { bounds, links, triangles, nodeCount } = data;
  if (!(bounds instanceof Float32Array) || !(links instanceof Uint32Array) || !(triangles instanceof Float32Array) || typeof nodeCount !== 'number') return null;
  return { bounds, links, triangles, nodeCount };
}

/** Builds in a worker; where workers cannot start, on this thread, which costs a short pause once. */
function buildOffThread(vertices: Float32Array, triangles: Uint32Array): Promise<SightBvh> {
  let worker: Worker;
  try {
    worker = new Worker(new URL('./sight-worker.ts', import.meta.url));
  } catch {
    return Promise.resolve(buildSightBvh(vertices, triangles));
  }
  return new Promise((resolve) => {
    const local = (): void => {
      worker.terminate();
      resolve(buildSightBvh(vertices, triangles));
    };
    worker.addEventListener('message', (event: MessageEvent<unknown>) => {
      const bvh = asBvh(event.data);
      if (bvh === null) {
        local();
        return;
      }
      worker.terminate();
      resolve(bvh);
    });
    worker.addEventListener('error', local);
    // Copies, so the 3D view's cached mesh keeps its own arrays.
    const copy = { vertices: vertices.slice(), triangles: triangles.slice() };
    worker.postMessage(copy, [copy.vertices.buffer, copy.triangles.buffer]);
  });
}

async function load(map: string): Promise<SightGeometry> {
  try {
    const geometry = await loadMapGeometry(map);
    // The walkable floor alone has no walls: every pair would look exposed.
    if (geometry.source !== 'physics') return { kind: 'failed', message: NO_WALLS };
    return { kind: 'ready', bvh: await buildOffThread(geometry.vertices, geometry.triangles) };
  } catch {
    return { kind: 'failed', message: NO_WALLS };
  }
}

export function loadSightGeometry(map: string): Promise<SightGeometry> {
  const cached = cache.get(map);
  if (cached !== undefined) return cached;
  const pending = load(map);
  cache.set(map, pending);
  void pending.then((result) => {
    if (result.kind === 'failed' && cache.get(map) === pending) cache.delete(map);
  });
  return pending;
}
