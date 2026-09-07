import { sqliteTable, text, integer, primaryKey } from "drizzle-orm/sqlite-core";
import type { AdapterAccountType } from "next-auth/adapters";

// Auth.js's own required tables (schema shape mandated by @auth/drizzle-adapter).

export const users = sqliteTable("user", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  name: text("name"),
  email: text("email").unique(),
  emailVerified: integer("emailVerified", { mode: "timestamp_ms" }),
  image: text("image"),
});

export const accounts = sqliteTable(
  "account",
  {
    userId: text("userId")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    type: text("type").$type<AdapterAccountType>().notNull(),
    provider: text("provider").notNull(),
    providerAccountId: text("providerAccountId").notNull(),
    refresh_token: text("refresh_token"),
    access_token: text("access_token"),
    expires_at: integer("expires_at"),
    token_type: text("token_type"),
    scope: text("scope"),
    id_token: text("id_token"),
    session_state: text("session_state"),
  },
  (account) => [
    primaryKey({ columns: [account.provider, account.providerAccountId] }),
  ],
);

export const sessions = sqliteTable("session", {
  sessionToken: text("sessionToken").primaryKey(),
  userId: text("userId")
    .notNull()
    .references(() => users.id, { onDelete: "cascade" }),
  expires: integer("expires", { mode: "timestamp_ms" }).notNull(),
});

export const verificationTokens = sqliteTable(
  "verificationToken",
  {
    identifier: text("identifier").notNull(),
    token: text("token").notNull(),
    expires: integer("expires", { mode: "timestamp_ms" }).notNull(),
  },
  (vt) => [primaryKey({ columns: [vt.identifier, vt.token] })],
);

// ClipHub Portal's own domain table. Kept deliberately minimal for the Phase 1
// walking skeleton: approval, the bridge hookup, and multi-artifact review are
// later phases and add columns then rather than guessing the shape now.
export const REQUEST_STATUSES = [
  "awaiting_demo", // row created client-side, demo upload not finished yet
  "pending", // demo uploaded, awaiting owner review in /admin
  "approved", // owner approved; not yet claimed by the local bridge
  "processing", // claimed by the bridge / being worked in local Studio
  "done", // finalVideoPath is ready to download
  "failed", // failureReason explains why
  "rejected", // owner declined the request
] as const;

export type RequestStatus = (typeof REQUEST_STATUSES)[number];

export const requests = sqliteTable("request", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  userId: text("userId")
    .notNull()
    .references(() => users.id, { onDelete: "cascade" }),
  status: text("status").notNull().default("awaiting_demo"),
  note: text("note"),
  demoPath: text("demoPath"),
  demoSha256: text("demoSha256"),
  demoOriginalName: text("demoOriginalName"),
  finalVideoPath: text("finalVideoPath"),
  finalVideoName: text("finalVideoName"),
  failureReason: text("failureReason"),
  createdAt: integer("createdAt", { mode: "timestamp_ms" })
    .notNull()
    .$defaultFn(() => new Date()),
  updatedAt: integer("updatedAt", { mode: "timestamp_ms" })
    .notNull()
    .$defaultFn(() => new Date()),
});
