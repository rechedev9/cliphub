import { createHash } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdir, rm } from "node:fs/promises";
import { dirname } from "node:path";
import { Readable, Transform } from "node:stream";
import type { ReadableStream as NodeWebReadableStream } from "node:stream/web";
import { pipeline } from "node:stream/promises";

export class UploadTooLargeError extends Error {}
export class InvalidContentError extends Error {}

export interface StreamUploadResult {
  bytesWritten: number;
  sha256: string;
}

export interface StreamUploadOptions {
  // Inspects the first bytes of the stream. Returning false aborts the
  // upload and removes the partial file. Omit it to accept any content
  // (a rendered reel has no signature worth checking here).
  validateHeader?: (header: Buffer) => boolean;
  // How many leading bytes validateHeader needs before it can decide.
  headerBytes?: number;
}

// Streams a request body straight to disk: never buffers the whole upload in
// memory, enforces maxBytes while writing, hashes as it goes, and optionally
// rejects content whose first bytes fail validateHeader — all without reading
// the stream twice. On any failure the partial file is removed.
export async function streamUploadToFile(
  body: NodeWebReadableStream<Uint8Array>,
  destPath: string,
  maxBytes: number,
  options: StreamUploadOptions = {},
): Promise<StreamUploadResult> {
  await mkdir(dirname(destPath), { recursive: true });

  const { validateHeader, headerBytes = 8 } = options;
  const hash = createHash("sha256");
  let bytesWritten = 0;
  let headerChecked = validateHeader === undefined;
  let headerBuf = Buffer.alloc(0);

  const guard = new Transform({
    transform(chunk: Buffer, _encoding, callback) {
      bytesWritten += chunk.length;
      if (bytesWritten > maxBytes) {
        callback(new UploadTooLargeError(`upload exceeds ${maxBytes} bytes`));
        return;
      }
      if (!headerChecked && validateHeader) {
        headerBuf = Buffer.concat([headerBuf, chunk]).subarray(0, headerBytes);
        if (headerBuf.length >= headerBytes - 1) {
          headerChecked = true;
          if (!validateHeader(headerBuf)) {
            callback(new InvalidContentError("content failed header validation"));
            return;
          }
        }
      }
      hash.update(chunk);
      callback(null, chunk);
    },
  });

  const source = Readable.fromWeb(body);
  const dest = createWriteStream(destPath);

  try {
    await pipeline(source, guard, dest);
  } catch (err) {
    await rm(destPath, { force: true });
    throw err;
  }

  if (!headerChecked) {
    // The whole stream was shorter than the header the validator needs.
    await rm(destPath, { force: true });
    throw new InvalidContentError("content failed header validation");
  }

  return { bytesWritten, sha256: hash.digest("hex") };
}
