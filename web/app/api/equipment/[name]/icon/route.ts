import { NextResponse } from 'next/server';
import { isEquipmentIconName } from '@/lib/api/game-assets';
import { orchestratorUrl, proxyGameAsset } from '../../../demos/_lib';

export const runtime = 'nodejs';

/**
 * GET /api/equipment/{name}/icon: CS2's icon for a weapon or piece of gear, read from the local
 * CS2 install as sanitized SVG (or PNG). proxy.ts runs the local-request guard.
 */
export async function GET(_request: Request, { params }: { params: Promise<{ name: string }> }): Promise<Response> {
  const { name } = await params;
  if (!isEquipmentIconName(name)) {
    return NextResponse.json({ error: 'invalid icon name', code: 'invalid_request' }, { status: 400 });
  }
  return proxyGameAsset(`${orchestratorUrl()}/api/equipment/${name}/icon`);
}
