import { createExtractorFromData } from 'node-unrar-js';
import { archiveEntryBaseName, isDemoFileName, isIgnoredArchivePath } from './demo-names.ts';

const UNRAR_WASM_PATH = '/unrar.wasm';
const OCTET_STREAM = 'application/octet-stream';

class ArchivePasswordError extends Error {
  constructor() {
    super('password protected archive');
    this.name = 'ArchivePasswordError';
  }
}

export async function extractRarDemos(data: ArrayBuffer): Promise<File[]> {
  const wasmBinary = await loadUnrarWasm();
  const extractor = await createExtractorFromData(wasmBinary === undefined ? { data } : { data, wasmBinary });
  const extracted = extractor.extract({
    files: (header) =>
      !header.flags.directory
      && !isIgnoredArchivePath(header.name)
      && isDemoFileName(archiveEntryBaseName(header.name)),
  });
  const files: File[] = [];
  for (const file of extracted.files) {
    if (file.fileHeader.flags.encrypted) throw new ArchivePasswordError();
    const bytes = file.extraction;
    if (bytes === undefined) continue;
    files.push(fileFromBytes(archiveEntryBaseName(file.fileHeader.name), bytes));
  }
  return files;
}

function fileFromBytes(name: string, data: Uint8Array): File {
  const copy = new Uint8Array(data.byteLength);
  copy.set(data);
  return new File([copy], name, { type: OCTET_STREAM });
}

async function loadUnrarWasm(): Promise<ArrayBuffer | undefined> {
  if (typeof window === 'undefined') return undefined;
  const res = await fetch(UNRAR_WASM_PATH);
  if (!res.ok) throw new Error(`failed to load unrar wasm (${res.status})`);
  return res.arrayBuffer();
}
