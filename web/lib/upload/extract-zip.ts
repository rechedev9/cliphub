import { unzipSync } from 'fflate';
import { archiveEntryBaseName, isDemoFileName, isIgnoredArchivePath } from './demo-names.ts';

const OCTET_STREAM = 'application/octet-stream';

export function extractZipDemos(data: Uint8Array): File[] {
  const unzipped = unzipSync(data, {
    filter: (file) => !isIgnoredArchivePath(file.name) && isDemoFileName(archiveEntryBaseName(file.name)),
  });
  return Object.entries(unzipped).map(([name, bytes]) => fileFromBytes(archiveEntryBaseName(name), bytes));
}

function fileFromBytes(name: string, data: Uint8Array): File {
  const copy = new Uint8Array(data.byteLength);
  copy.set(data);
  return new File([copy], name, { type: OCTET_STREAM });
}
