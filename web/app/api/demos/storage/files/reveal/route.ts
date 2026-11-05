import { localStorageFileAction } from '../../../_local';

export const runtime = 'nodejs';

/** POST /api/demos/storage/files/reveal {path}: open File Explorer on a listed file. */
export async function POST(request: Request): Promise<Response> {
  return localStorageFileAction(request, 'reveal');
}
