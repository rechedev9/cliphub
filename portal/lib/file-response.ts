import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";

import { parseRange } from "./range.ts";

export interface FileResponseOptions {
  request: Request;
  path: string;
  contentType: string;
  // Extra headers, for example ETag or Content-Disposition.
  headers?: Record<string, string>;
  // The answer when the file is no longer on disk.
  missing: Response;
}

interface StreamRange {
  start: number;
  end: number;
}

function fileStream(path: string, range: StreamRange): ReadableStream<Uint8Array> {
  const source = createReadStream(path, range);
  const iterator = source[Symbol.asyncIterator]();
  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      for (;;) {
        const next = await iterator.next();
        if (next.done) {
          controller.close();
          return;
        }
        if (next.value instanceof Uint8Array) {
          controller.enqueue(next.value);
          return;
        }
      }
    },
    cancel() {
      source.destroy();
    },
  });
}

// Serves a file with support for a single Range, so downloads can resume.
export async function fileResponse(options: FileResponseOptions): Promise<Response> {
  const { request, path, contentType } = options;
  const fileStat = await stat(path).catch(() => null);
  if (!fileStat || !fileStat.isFile()) return options.missing;

  const size = fileStat.size;
  const base = {
    ...options.headers,
    "Content-Type": contentType,
    "Accept-Ranges": "bytes",
  };
  const range = parseRange(request.headers.get("range"), size);
  if (range === "invalid") {
    return new Response(null, {
      status: 416,
      headers: { ...base, "Content-Range": `bytes */${size}` },
    });
  }
  if (range === null) {
    if (size === 0) return new Response(null, { headers: { ...base, "Content-Length": "0" } });
    return new Response(fileStream(path, { start: 0, end: size - 1 }), {
      headers: { ...base, "Content-Length": String(size) },
    });
  }
  return new Response(fileStream(path, range), {
    status: 206,
    headers: {
      ...base,
      "Content-Length": String(range.end - range.start + 1),
      "Content-Range": `bytes ${range.start}-${range.end}/${size}`,
    },
  });
}
