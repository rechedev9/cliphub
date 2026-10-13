import { timingSafeEqual } from "node:crypto";

import { and, desc, eq, gt, isNull } from "drizzle-orm";

import { db, transact } from "../db/client.ts";
import { deviceLinks, devices, users } from "../db/schema.ts";
import { recordEvent } from "./events.ts";
import { createRateLimiter } from "./rate-limit.ts";
import {
  formatUserCode,
  hashToken,
  newDeviceToken,
  newPollToken,
  newUserCode,
  normalizeUserCode,
} from "./tokens.ts";

const LINK_TTL_MS = 10 * 60 * 1000;
// An approval given in the last seconds can still be collected by the next polls.
const CONSUME_GRACE_MS = 5 * 60 * 1000;
const POLL_INTERVAL_SECONDS = 3;
const CODE_ATTEMPTS = 5;
const MAX_DEVICE_NAME_CHARS = 60;
// What clientIp answers when the proxy header is missing; it never counts as a match.
const UNKNOWN_IP = "unknown";

// Link start is open to anyone, so it is limited per address: 10 an hour.
export const linkStartLimiter = createRateLimiter({ limit: 10, windowMs: 60 * 60 * 1000 });
// Code lookups and approvals per signed-in user, so codes cannot be guessed in bulk.
export const linkCodeLimiter = createRateLimiter({ limit: 30, windowMs: LINK_TTL_MS });

// The device name Studio sends (its hostname), or null when it is empty.
export function parseDeviceName(input: unknown): string | null {
  if (typeof input !== "string") return null;
  const name = input.trim().slice(0, MAX_DEVICE_NAME_CHARS);
  return name.length > 0 ? name : null;
}

// The address is stored hashed, salted with the link's own secret, only to compare it later.
function networkHash(pollTokenHash: string, ip: string): string | null {
  return ip === UNKNOWN_IP ? null : hashToken(`${pollTokenHash}:${ip}`);
}

export interface StartLinkInput {
  deviceName: string;
  // The address the request came from, as clientIp reads it.
  ip: string;
  // The public origin of the portal, for the URL the user opens.
  origin: string;
  now: number;
}

export interface StartedLink {
  linkId: string;
  pollToken: string;
  userCode: string;
  verifyUrl: string;
  expiresAt: number;
  intervalSeconds: number;
}

async function unusedUserCode(): Promise<string> {
  for (let attempt = 0; attempt < CODE_ATTEMPTS; attempt += 1) {
    const code = newUserCode();
    const [taken] = await db
      .select({ id: deviceLinks.id })
      .from(deviceLinks)
      .where(eq(deviceLinks.userCode, code));
    if (!taken) return code;
  }
  throw new Error("could not allocate a link code");
}

// Step 1 of linking: Studio asks for a code to show and a secret to poll with.
export async function startLink(input: StartLinkInput): Promise<StartedLink> {
  const { now } = input;
  const pollToken = newPollToken();
  const pollTokenHash = hashToken(pollToken);
  const userCode = await unusedUserCode();
  const expiresAt = now + LINK_TTL_MS;
  const [link] = await db
    .insert(deviceLinks)
    .values({
      userCode,
      pollTokenHash,
      deviceName: input.deviceName,
      createdAt: now,
      expiresAt,
      startIpHash: networkHash(pollTokenHash, input.ip),
    })
    .returning({ id: deviceLinks.id });
  if (!link) throw new Error("could not create the link");

  const shown = formatUserCode(userCode);
  return {
    linkId: link.id,
    pollToken,
    userCode: shown,
    verifyUrl: `${input.origin}/link?code=${shown}`,
    expiresAt,
    intervalSeconds: POLL_INTERVAL_SECONDS,
  };
}

function sameHash(a: string, b: string): boolean {
  const left = Buffer.from(a, "utf8");
  const right = Buffer.from(b, "utf8");
  return left.length === right.length && timingSafeEqual(left, right);
}

export type PollResult =
  | { status: "pending" | "denied" | "expired" }
  | {
      status: "approved";
      deviceToken: string;
      user: { name: string | null; email: string | null };
    };

export interface PollInput {
  linkId: string;
  pollToken: string;
  now: number;
}

// Step 3: Studio collects its device token. It is handed out exactly once.
export async function pollLink(input: PollInput): Promise<PollResult> {
  const { now } = input;
  const [link] = await db.select().from(deviceLinks).where(eq(deviceLinks.id, input.linkId));
  // An unknown link and a wrong secret look the same: the caller has to start over either way.
  if (!link || !sameHash(link.pollTokenHash, hashToken(input.pollToken))) {
    return { status: "expired" };
  }
  if (link.status === "denied") return { status: "denied" };
  if (link.status === "pending") {
    return { status: link.expiresAt < now ? "expired" : "pending" };
  }
  const userId = link.userId;
  if (link.status !== "approved" || !userId || link.expiresAt + CONSUME_GRACE_MS < now) {
    return { status: "expired" };
  }

  return transact(async (tx): Promise<PollResult> => {
    const consumed = await tx
      .update(deviceLinks)
      .set({ status: "consumed" })
      .where(and(eq(deviceLinks.id, link.id), eq(deviceLinks.status, "approved")))
      .returning({ id: deviceLinks.id });
    if (consumed.length === 0) return { status: "expired" };

    const deviceToken = newDeviceToken();
    await tx.insert(devices).values({
      userId,
      name: link.deviceName,
      tokenHash: hashToken(deviceToken),
      createdAt: now,
    });
    await recordEvent(
      {
        type: "device_linked",
        actor: `user:${userId}`,
        at: now,
        subjectUserId: userId,
        detail: link.deviceName,
      },
      tx,
    );
    const [user] = await tx
      .select({ name: users.name, email: users.email })
      .from(users)
      .where(eq(users.id, userId));
    return {
      status: "approved",
      deviceToken,
      user: { name: user?.name ?? null, email: user?.email ?? null },
    };
  });
}

async function pendingLink(userCode: string, now: number) {
  const code = normalizeUserCode(userCode);
  if (code === null) return null;
  const [link] = await db
    .select()
    .from(deviceLinks)
    .where(
      and(
        eq(deviceLinks.userCode, code),
        eq(deviceLinks.status, "pending"),
        gt(deviceLinks.expiresAt, now),
      ),
    );
  return link ?? null;
}

type LinkRow = typeof deviceLinks.$inferSelect;

// True when the browser is at the address that asked for the code: Studio on the same connection.
function sameNetwork(link: LinkRow, ip: string): boolean {
  const hash = networkHash(link.pollTokenHash, ip);
  return hash !== null && link.startIpHash !== null && sameHash(hash, link.startIpHash);
}

export interface LinkDescription {
  deviceName: string;
  expiresAt: number;
  sameNetwork: boolean;
}

export interface DescribeLinkInput {
  userCode: string;
  // The browser's address.
  ip: string;
  now: number;
}

// What the confirmation page shows for a code, or null when it is unknown or expired.
export async function describeLink(input: DescribeLinkInput): Promise<LinkDescription | null> {
  const link = await pendingLink(input.userCode, input.now);
  if (!link) return null;
  return {
    deviceName: link.deviceName,
    expiresAt: link.expiresAt,
    sameNetwork: sameNetwork(link, input.ip),
  };
}

export interface DecideLinkInput {
  userCode: string;
  approve: boolean;
  userId: string;
  ip: string;
  // The code as the user copied it from Studio by hand; needed from another network.
  typedCode?: string | null;
  now: number;
}

export type LinkDecision = "ok" | "not_found" | "code_confirmation_required";

// Step 2: the signed-in user confirms (or refuses) the device.
export async function decideLink(input: DecideLinkInput): Promise<LinkDecision> {
  const link = await pendingLink(input.userCode, input.now);
  if (!link) return "not_found";
  // A code that reached the user in a link from somewhere else proves nothing: they must
  // read it off their own Studio. Refusing never needs it.
  if (input.approve && !sameNetwork(link, input.ip)) {
    const typed = input.typedCode ? normalizeUserCode(input.typedCode) : null;
    if (typed !== link.userCode) return "code_confirmation_required";
  }
  const decided = await db
    .update(deviceLinks)
    .set({ status: input.approve ? "approved" : "denied", userId: input.userId })
    .where(and(eq(deviceLinks.id, link.id), eq(deviceLinks.status, "pending")))
    .returning({ id: deviceLinks.id });
  return decided.length > 0 ? "ok" : "not_found";
}

export interface DeviceView {
  id: string;
  name: string;
  createdAt: number;
  lastSeenAt: number | null;
}

export async function listDevices(userId: string): Promise<DeviceView[]> {
  return db
    .select({
      id: devices.id,
      name: devices.name,
      createdAt: devices.createdAt,
      lastSeenAt: devices.lastSeenAt,
    })
    .from(devices)
    .where(and(eq(devices.userId, userId), isNull(devices.revokedAt)))
    .orderBy(desc(devices.createdAt));
}

export interface RevokeDeviceInput {
  deviceId: string;
  userId: string;
  now: number;
}

// Revokes one of the user's own devices. False when it is not theirs or already revoked.
export async function revokeDevice(input: RevokeDeviceInput): Promise<boolean> {
  const { deviceId, userId, now } = input;
  return transact(async (tx) => {
    const revoked = await tx
      .update(devices)
      .set({ revokedAt: now })
      .where(
        and(eq(devices.id, deviceId), eq(devices.userId, userId), isNull(devices.revokedAt)),
      )
      .returning({ name: devices.name });
    const [device] = revoked;
    if (!device) return false;
    await recordEvent(
      {
        type: "device_revoked",
        actor: `user:${userId}`,
        at: now,
        subjectUserId: userId,
        detail: device.name,
      },
      tx,
    );
    return true;
  });
}
