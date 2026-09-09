// node --expose-gc web/scripts/bench-tactical-decode.mjs <index.json> <positions.bin>
// Uses a verified real scan, not a synthetic estimate of decoded object sizes.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { decodePositionsHeader, decodeRoundFrames } from '../lib/tactical-decode.ts';
import { LruCache } from '../lib/lru-cache.ts';

const [indexPath, blobPath] = process.argv.slice(2);
if (!indexPath || !blobPath) throw new Error('usage: bench-tactical-decode.mjs <index.json> <positions.bin>');
const doc = JSON.parse(readFileSync(indexPath, 'utf8'));
const bytes = readFileSync(blobPath);
assert.equal(createHash('sha256').update(bytes).digest('hex'), doc.positions.sha256);
const blob = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
const scale = decodePositionsHeader(blob);
const offsets = doc.positions.round_offsets;
const percentile = (values, p) => values.toSorted((a,b) => a-b)[Math.max(0, Math.ceil(values.length*p/100)-1)];

function measure(cache) {
  global.gc?.();
  const startHeap = process.memoryUsage().heapUsed;
  const durations = [];
  for (const offset of offsets) {
    const start = performance.now();
    cache.set(offset.round, decodeRoundFrames(blob, offset, scale));
    durations.push(performance.now()-start);
  }
  global.gc?.();
  const retained = global.gc ? process.memoryUsage().heapUsed - startHeap : null;
  const result = { retained_rounds: cache.size, retained_heap_bytes: retained,
    decode_round_p95_ms: percentile(durations,95), decode_round_p99_ms: percentile(durations,99) };
  cache.clear();
  return result;
}
// Warm JIT, then compare retained caches. Timings are Node decoder latency,
// not renderer long tasks; a Web Worker requires separate browser evidence.
measure(new LruCache(4));
console.log(JSON.stringify({ scenario:'tactical-round-decode', rounds:offsets.length, blob_bytes:blob.byteLength,
  baseline:measure(new Map()), candidate:measure(new LruCache(4)),
},null,2));
