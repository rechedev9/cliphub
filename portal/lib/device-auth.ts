import { and, eq, isNull } from "drizzle-orm";

import { db } from "../db/client.ts";
import { devices } from "../db/schema.ts";
import { hashToken, isDeviceToken } from "./tokens.ts";

const LAST_SEEN_WRITE_INTERVAL_MS = 60 * 1000;

export interface DeviceIdentity {
  deviceId: string;
  userId: string;
}

// The linked Studio install behind a chd_ token, or null for an unknown or revoked one.
export async function authenticateDevice(
  token: string,
  now: number,
): Promise<DeviceIdentity | null> {
  if (!isDeviceToken(token)) return null;
  const [device] = await db
    .select()
    .from(devices)
    .where(and(eq(devices.tokenHash, hashToken(token)), isNull(devices.revokedAt)));
  if (!device) return null;
  // Studio polls every few seconds; one write a minute is enough for "last seen".
  if (device.lastSeenAt === null || now - device.lastSeenAt > LAST_SEEN_WRITE_INTERVAL_MS) {
    await db.update(devices).set({ lastSeenAt: now }).where(eq(devices.id, device.id));
  }
  return { deviceId: device.id, userId: device.userId };
}
