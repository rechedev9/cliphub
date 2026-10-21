import { localStorageUsage } from '../_local';

export const runtime = 'nodejs';

/** GET /api/demos/storage: disk use of every Studio job folder, for the Ajustes storage panel. */
export async function GET(): Promise<Response> {
  return localStorageUsage();
}
