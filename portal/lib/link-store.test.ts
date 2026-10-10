import { NOW, MINUTE, makeUser, resetDatabase } from "./test-support.ts";

import { beforeEach, test } from "node:test";
import assert from "node:assert/strict";

import { db } from "../db/client.ts";
import { deviceLinks } from "../db/schema.ts";
import { authenticateDevice } from "./device-auth.ts";
import {
  decideLink,
  describeLink,
  listDevices,
  parseDeviceName,
  pollLink,
  revokeDevice,
  startLink,
} from "./link-store.ts";
import { createRateLimiter } from "./rate-limit.ts";

const ORIGIN = "https://cliphub.gravityroom.app";
// Studio and the browser that confirms normally share one public address.
const HOME = "203.0.113.7";
const ELSEWHERE = "198.51.100.20";

beforeEach(resetDatabase);

async function start(ip = HOME) {
  return startLink({ deviceName: "DESKTOP-ABC", ip, origin: ORIGIN, now: NOW });
}

function describe(userCode: string, now = NOW) {
  return describeLink({ userCode, ip: HOME, now });
}

test("starting a link returns a code to show, a URL to open and a secret to poll with", async () => {
  const link = await start();
  assert.match(link.userCode, /^[A-Z2-9]{4}-[A-Z2-9]{4}$/);
  assert.equal(link.verifyUrl, `${ORIGIN}/link?code=${link.userCode}`);
  assert.equal(link.expiresAt, NOW + 10 * MINUTE);
  assert.equal(link.intervalSeconds, 3);
  assert.match(link.pollToken, /^[0-9a-f]{64}$/);
  assert.deepEqual(await pollLink({ linkId: link.linkId, pollToken: link.pollToken, now: NOW }), {
    status: "pending",
  });
});

test("an approved link hands out a working device token exactly once", async () => {
  const userId = await makeUser("luis");
  const link = await start();
  const poll = { linkId: link.linkId, pollToken: link.pollToken, now: NOW + 5000 };

  assert.deepEqual(await describe(link.userCode.toLowerCase()), {
    deviceName: "DESKTOP-ABC",
    expiresAt: link.expiresAt,
    sameNetwork: true,
  });
  assert.equal(await decideLink({ userCode: link.userCode, approve: true, userId, ip: HOME, now: NOW + 1000 }), "ok");

  const approved = await pollLink(poll);
  assert.equal(approved.status, "approved");
  if (approved.status !== "approved") return;
  assert.match(approved.deviceToken, /^chd_[0-9a-f]{64}$/);
  assert.deepEqual(approved.user, { name: "luis", email: "luis@example.test" });

  const identity = await authenticateDevice(approved.deviceToken, NOW + 6000);
  assert.equal(identity?.userId, userId);
  assert.deepEqual(await pollLink(poll), { status: "expired" });
  assert.equal((await listDevices(userId)).length, 1);
  assert.equal(await describe(link.userCode), null);
});

test("a refused link never yields a token", async () => {
  const userId = await makeUser("luis");
  const link = await start();
  assert.equal(await decideLink({ userCode: link.userCode, approve: false, userId, ip: HOME, now: NOW }), "ok");
  assert.deepEqual(await pollLink({ linkId: link.linkId, pollToken: link.pollToken, now: NOW }), {
    status: "denied",
  });
  assert.equal(await decideLink({ userCode: link.userCode, approve: true, userId, ip: HOME, now: NOW }), "not_found");
  assert.deepEqual(await listDevices(userId), []);
});

test("a code cannot be approved after its ten minutes", async () => {
  const userId = await makeUser("luis");
  const link = await start();
  const late = NOW + 10 * MINUTE + 1;

  assert.equal(await describe(link.userCode, late), null);
  assert.equal(await decideLink({ userCode: link.userCode, approve: true, userId, ip: HOME, now: late }), "not_found");
  assert.deepEqual(await pollLink({ linkId: link.linkId, pollToken: link.pollToken, now: late }), {
    status: "expired",
  });
});

test("the poll secret is required: knowing the link id is not enough", async () => {
  const userId = await makeUser("luis");
  const link = await start();
  await decideLink({ userCode: link.userCode, approve: true, userId, ip: HOME, now: NOW });

  const guess = await pollLink({ linkId: link.linkId, pollToken: "0".repeat(64), now: NOW });
  assert.deepEqual(guess, { status: "expired" });
  assert.deepEqual(await listDevices(userId), []);
  const real = await pollLink({ linkId: link.linkId, pollToken: link.pollToken, now: NOW });
  assert.equal(real.status, "approved");
});

test("an unknown code or link answers like an expired one", async () => {
  const userId = await makeUser("luis");
  assert.equal(await describe("AAAA-BBBB"), null);
  assert.equal(await describe("not a code"), null);
  assert.equal(await decideLink({ userCode: "AAAA-BBBB", approve: true, userId, ip: HOME, now: NOW }), "not_found");
  assert.deepEqual(await pollLink({ linkId: "missing", pollToken: "x", now: NOW }), { status: "expired" });
});

test("a revoked device stops authenticating, and only its owner can revoke it", async () => {
  const userId = await makeUser("luis");
  const stranger = await makeUser("stranger");
  const link = await start();
  await decideLink({ userCode: link.userCode, approve: true, userId, ip: HOME, now: NOW });
  const approved = await pollLink({ linkId: link.linkId, pollToken: link.pollToken, now: NOW });
  if (approved.status !== "approved") throw new Error("link not approved");
  const [device] = await listDevices(userId);
  if (!device) throw new Error("device not listed");

  assert.equal(await revokeDevice({ deviceId: device.id, userId: stranger, now: NOW }), false);
  assert.notEqual(await authenticateDevice(approved.deviceToken, NOW), null);

  assert.equal(await revokeDevice({ deviceId: device.id, userId, now: NOW }), true);
  assert.equal(await authenticateDevice(approved.deviceToken, NOW), null);
  assert.deepEqual(await listDevices(userId), []);
});

test("a token that is not a device token is never looked up", async () => {
  assert.equal(await authenticateDevice("a".repeat(64), NOW), null);
  assert.equal(await authenticateDevice("chd_short", NOW), null);
});

test("the device name is trimmed, capped at 60 characters and must not be empty", () => {
  assert.equal(parseDeviceName("  DESKTOP-ABC  "), "DESKTOP-ABC");
  assert.equal(parseDeviceName("x".repeat(100))?.length, 60);
  assert.equal(parseDeviceName("   "), null);
  assert.equal(parseDeviceName(42), null);
});

test("the limiter allows 10 attempts an hour per key and frees them as they age out", () => {
  const limiter = createRateLimiter({ limit: 10, windowMs: 60 * MINUTE });
  for (let attempt = 0; attempt < 10; attempt += 1) {
    assert.equal(limiter.allow("1.2.3.4", NOW + attempt), true);
  }
  assert.equal(limiter.allow("1.2.3.4", NOW + 100), false);
  assert.equal(limiter.allow("5.6.7.8", NOW + 100), true);
  assert.equal(limiter.allow("1.2.3.4", NOW + 60 * MINUTE), true);
  assert.equal(limiter.allow("1.2.3.4", NOW + 60 * MINUTE), false);
});

test("a code opened from another network cannot be approved without typing it from Studio", async () => {
  const victim = await makeUser("victim");
  // The attacker starts the link at their address and sends the victim the portal's own URL.
  const link = await start(ELSEWHERE);
  const poll = { linkId: link.linkId, pollToken: link.pollToken, now: NOW + 5000 };
  const decide = (typedCode: string | null) =>
    decideLink({ userCode: link.userCode, approve: true, userId: victim, ip: HOME, typedCode, now: NOW });

  assert.equal((await describeLink({ userCode: link.userCode, ip: HOME, now: NOW }))?.sameNetwork, false);
  assert.equal(await decide(null), "code_confirmation_required");
  assert.equal(await decide(""), "code_confirmation_required");
  assert.equal(await decide("AAAA-BBBB"), "code_confirmation_required");
  assert.deepEqual(await pollLink(poll), { status: "pending" });
  assert.deepEqual(await listDevices(victim), []);

  // Someone who really has Studio in front of them, on another connection, types what it shows.
  assert.equal(await decide(link.userCode.toLowerCase().replace("-", " ")), "ok");
  assert.equal((await pollLink(poll)).status, "approved");
});

test("refusing a code from another network is always one step", async () => {
  const userId = await makeUser("luis");
  const link = await start(ELSEWHERE);
  const refused = await decideLink({ userCode: link.userCode, approve: false, userId, ip: HOME, now: NOW });
  assert.equal(refused, "ok");
  assert.deepEqual(await pollLink({ linkId: link.linkId, pollToken: link.pollToken, now: NOW }), {
    status: "denied",
  });
});

test("without the proxy's address nothing counts as the same network", async () => {
  const userId = await makeUser("luis");
  const link = await start("unknown");
  const input = { userCode: link.userCode, approve: true, userId, ip: "unknown", now: NOW };

  assert.equal((await describeLink({ userCode: link.userCode, ip: "unknown", now: NOW }))?.sameNetwork, false);
  assert.equal(await decideLink(input), "code_confirmation_required");
  assert.equal(await decideLink({ ...input, typedCode: link.userCode }), "ok");
});

test("the address that started a link is stored only as a salted hash", async () => {
  const first = await start();
  const second = await start();
  const rows = await db.select().from(deviceLinks);
  const hashes = rows.map((row) => row.startIpHash);
  assert.equal(rows.length, 2);
  assert.notEqual(first.linkId, second.linkId);
  assert.equal(hashes.every((hash) => hash !== null && /^[0-9a-f]{64}$/.test(hash)), true);
  assert.notEqual(hashes[0], hashes[1]);
  assert.equal(JSON.stringify(rows).includes(HOME), false);
});
