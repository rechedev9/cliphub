// Local test data so the worker and Studio can be exercised without OAuth: node scripts/dev-seed.ts
// Every run adds new credentials, because only their hashes are stored.
import "./dev-seed-env.ts";

import { randomBytes } from "node:crypto";

import { and, eq } from "drizzle-orm";

import { db, rawClient } from "../db/client.ts";
import { runMigrations } from "../db/migrate.ts";
import { accounts, devices, sessions, userCloud, users, workers } from "../db/schema.ts";
import { hashToken, newDeviceToken, newWorkerToken } from "../lib/tokens.ts";

const DAY_MS = 24 * 60 * 60 * 1000;
// With ADMIN_ACCOUNTS=google:0 the first seeded user is also an admin.
const ADMIN_PROVIDER = "google";
const ADMIN_ACCOUNT_ID = "0";

async function ensureUser(email: string, name: string): Promise<string> {
  const [existing] = await db.select({ id: users.id }).from(users).where(eq(users.email, email));
  if (existing) return existing.id;
  const [created] = await db.insert(users).values({ email, name }).returning({ id: users.id });
  if (!created) throw new Error(`could not create ${email}`);
  return created.id;
}

async function allowCloud(userId: string): Promise<void> {
  const row = { access: "allowed", updatedAt: Date.now() };
  await db
    .insert(userCloud)
    .values({ userId, ...row })
    .onConflictDoUpdate({ target: userCloud.userId, set: row });
}

async function ensureAdminAccount(userId: string): Promise<void> {
  const [existing] = await db
    .select({ userId: accounts.userId })
    .from(accounts)
    .where(
      and(eq(accounts.provider, ADMIN_PROVIDER), eq(accounts.providerAccountId, ADMIN_ACCOUNT_ID)),
    );
  if (existing) return;
  await db.insert(accounts).values({
    userId,
    type: "oidc",
    provider: ADMIN_PROVIDER,
    providerAccountId: ADMIN_ACCOUNT_ID,
  });
}

async function newDevice(userId: string, name: string): Promise<string> {
  const token = newDeviceToken();
  await db.insert(devices).values({
    userId,
    name,
    tokenHash: hashToken(token),
    createdAt: Date.now(),
  });
  return token;
}

async function newWorker(name: string): Promise<string> {
  const token = newWorkerToken();
  await db.insert(workers).values({ name, tokenHash: hashToken(token), createdAt: Date.now() });
  return token;
}

async function newSession(userId: string): Promise<string> {
  const sessionToken = randomBytes(32).toString("hex");
  await db.insert(sessions).values({
    sessionToken,
    userId,
    expires: new Date(Date.now() + 30 * DAY_MS),
  });
  return sessionToken;
}

await runMigrations();

const devUser = await ensureUser("dev@cliphub.test", "Dev");
const secondUser = await ensureUser("second@cliphub.test", "Second");
await allowCloud(devUser);
await allowCloud(secondUser);
await ensureAdminAccount(devUser);

const lines = [
  `DATABASE=${process.env.DATABASE_URL}`,
  `DEVICE=${await newDevice(devUser, "dev-seed")}`,
  `DEVICE_2=${await newDevice(secondUser, "dev-seed-2")}`,
  `WORKER=${await newWorker("dev-worker")}`,
  `SESSION_COOKIE=authjs.session-token=${await newSession(devUser)}`,
  `SESSION_COOKIE_2=authjs.session-token=${await newSession(secondUser)}`,
];
console.log(lines.join("\n"));
console.log(
  [
    "",
    "DEVICE: token de Studio del usuario dev@cliphub.test (Authorization: Bearer ...).",
    "DEVICE_2: token de un segundo usuario, para comprobar que no ve los jobs del primero.",
    "WORKER: valor de ZV_BRIDGE_TOKEN para el worker.",
    "SESSION_COOKIE: sesión de navegador del usuario dev. Con ADMIN_ACCOUNTS=google:0 es admin.",
    "SESSION_COOKIE_2: sesión de navegador del segundo usuario, que no es admin.",
    "  Las peticiones que no son GET necesitan además la cabecera Origin igual a AUTH_URL.",
  ].join("\n"),
);

rawClient.close();
