import { createHash } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdir, rm } from "node:fs/promises";
import { dirname } from "node:path";
import { Readable, Transform } from "node:stream";
import type { ReadableStream as NodeWebReadableStream } from "node:stream/web";
import { pipeline } from "node:stream/promises";

import { isDemoHeader } from "./demo-validate.ts";

export class UploadTooLargeError extends Error {}
export class InvalidDemoError extends Error {}

export interface StreamUploadResult {
  bytesWritten: number;
  sha256: string;
}

// Streams a request body straight to disk: never buffers the whole upload in
// memory, enforces maxBytes while writing, hashes as it goes, and rejects
// anything whose first bytes aren't a real CS2/GOTV demo header — all without
// reading the stream twice. On any failure the partial file is removed.
export async function streamUploadToFile(
  body: NodeWebReadableStream<Uint8Array>,
  destPath: string,
  maxBytes: number,
): Promise<StreamUploadResult> {
  await mkdir(dirname(destPath), { recursive: true });

  const hash = createHash("sha256");
  let bytesWritten = 0;
  let headerChecked = false;
  let headerBuf = Buffer.alloc(0);

  const guard = new Transform({
    transform(chunk: Buffer, _encoding, callback) {
      bytesWritten += chunk.length;
      if (bytesWritten > maxBytes) {
        callback(new UploadTooLargeError(`upload exceeds ${maxBytes} bytes`));
        return;
      }
      if (!headerChecked) {
        headerBuf = Buffer.concat([headerBuf, chunk]).subarray(0, 8);
        if (headerBuf.length >= 7) {
          headerChecked = true;
          if (!isDemoHeader(headerBuf)) {
            callback(new InvalidDemoError("uploaded file is not a CS2 demo"));
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
    await rm(destPath, { force: true });
    throw new InvalidDemoError("uploaded file is not a CS2 demo");
  }

  return { bytesWritten, sha256: hash.digest("hex") };
}
