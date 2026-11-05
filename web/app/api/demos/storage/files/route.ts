import { localStorageFiles } from '../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/storage/files: every demo on the PC and every video Studio rendered. */
export async function GET(): Promise<Response> {
  return localStorageFiles();
}
