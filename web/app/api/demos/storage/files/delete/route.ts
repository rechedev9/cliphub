import { localStorageFileAction } from '../../../_local';

export const runtime = 'nodejs';

/** POST /api/demos/storage/files/delete {path}: delete a listed demo or video; 200 with the freed bytes. */
export async function POST(request: Request): Promise<Response> {
  return localStorageFileAction(request, 'delete');
}
