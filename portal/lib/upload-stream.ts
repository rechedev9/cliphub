import { createHash } from "node:crypto";
import { constants, createReadStream, createWriteStream } from "node:fs";
import { mkdir, open, rm } from "node:fs/promises";
import { dirname } from "node:path";
import { Readable, Transform } from "node:stream";
import { pipeline } from "node:stream/promises";

export class UploadTooLargeError extends Error {}
export class PartLengthError extends Error {}

export class InvalidContentError extends Error {
  // The leading bytes that failed validation, so the caller can say what the file was.
  readonly header: Buffer;

  constructor(header: Buffer) {
    super("content failed header validation");
    this.header = header;
  }
}

export interface StreamUploadResult {
  bytesWritten: number;
  sha256: string;
}

export interface StreamUploadOptions {
  body: ReadableStream<Uint8Array>;
  destPath: string;
  maxBytes: number;
  // Inspects the first bytes. Returning false aborts and removes the partial file.
  validateHeader?: (header: Buffer) => boolean;
  // How many leading bytes validateHeader needs before it can decide.
  headerBytes?: number;
}

async function* bodyChunks(body: ReadableStream<Uint8Array>): AsyncGenerator<Buffer> {
  const reader = body.getReader();
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return;
      yield Buffer.from(value.buffer, value.byteOffset, value.byteLength);
    }
  } finally {
    reader.releaseLock();
  }
}

// Streams a request body to disk without buffering it: enforces maxBytes while
// writing, hashes as it goes and can reject content by its first bytes.
export async function streamUploadToFile(
  options: StreamUploadOptions,
): Promise<StreamUploadResult> {
  const { body, destPath, maxBytes, validateHeader, headerBytes = 8 } = options;
  await mkdir(dirname(destPath), { recursive: true });

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
            callback(new InvalidContentError(headerBuf));
            return;
          }
        }
      }
      hash.update(chunk);
      callback(null, chunk);
    },
  });

  try {
    await pipeline(Readable.from(bodyChunks(body)), guard, createWriteStream(destPath));
  } catch (err) {
    await rm(destPath, { force: true });
    throw err;
  }

  if (!headerChecked) {
    // The whole stream was shorter than the header the validator needs.
    await rm(destPath, { force: true });
    throw new InvalidContentError(headerBuf);
  }

  return { bytesWritten, sha256: hash.digest("hex") };
}

export interface PartWriteOptions {
  body: ReadableStream<Uint8Array>;
  // Created when missing and never truncated, so parts can arrive in any order.
  path: string;
  offset: number;
  length: number;
}

// Writes one upload part at its offset. Throws PartLengthError unless the
// body is exactly `length` bytes, so a short part is never marked as received.
export async function writePartAt(options: PartWriteOptions): Promise<void> {
  const { body, path, offset, length } = options;
  const file = await open(path, constants.O_RDWR | constants.O_CREAT);
  let written = 0;
  try {
    for await (const chunk of bodyChunks(body)) {
      if (written + chunk.length > length) {
        throw new PartLengthError("part is longer than expected");
      }
      await file.write(chunk, 0, chunk.length, offset + written);
      written += chunk.length;
    }
  } finally {
    await file.close();
  }
  if (written !== length) throw new PartLengthError("part is shorter than expected");
}

export async function hashFile(path: string): Promise<string> {
  const hash = createHash("sha256");
  await pipeline(createReadStream(path), hash);
  return hash.digest("hex");
}

export interface ReadBytesOptions {
  path: string;
  start: number;
  length: number;
}

// Reads up to `length` bytes at `start`; shorter when the file ends first.
export async function readBytes(options: ReadBytesOptions): Promise<Buffer> {
  const file = await open(options.path, "r");
  try {
    const buffer = Buffer.alloc(options.length);
    const { bytesRead } = await file.read(buffer, 0, options.length, options.start);
    return buffer.subarray(0, bytesRead);
  } finally {
    await file.close();
  }
}
