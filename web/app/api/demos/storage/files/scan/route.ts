import { localScanStorageFiles } from '../../../_local';

export const runtime = 'nodejs';

/** POST /api/demos/storage/files/scan: search the PC for demos again. */
export async function POST(): Promise<Response> {
  return localScanStorageFiles();
}
