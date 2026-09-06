import { createHash } from 'node:crypto';
import { createReadStream, createWriteStream } from 'node:fs';
import { Transform } from 'node:stream';
import { pipeline } from 'node:stream/promises';

/** Copy and verify large bundled archives with bounded memory and backpressure.
 * The caller owns cleanup of the destination if copying is interrupted.
 */
export async function copyAndHash(source: string, destination: string, signal: AbortSignal): Promise<string> {
  const hash = createHash('sha256');
  await pipeline(
    createReadStream(source),
    new Transform({
      transform(chunk, _encoding, callback) {
        hash.update(chunk);
        callback(null, chunk);
      },
    }),
    createWriteStream(destination),
    { signal },
  );
  return hash.digest('hex');
}
