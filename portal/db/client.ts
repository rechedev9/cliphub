import { mkdirSync } from "node:fs";
import { dirname } from "node:path";

import { createClient } from "@libsql/client";
import { drizzle } from "drizzle-orm/libsql";

import * as schema from "./schema.ts";

const url = process.env.DATABASE_URL ?? "file:./data/portal.db";

// libsql opens the file when the client is constructed, and Auth.js's Drizzle
// adapter needs a concrete database at module scope, so this connection is
// unavoidably eager. `next build` imports every route module to collect its
// config, which means the build itself fails if the directory is missing —
// true of every clean deployment before a volume is mounted. Creating it
// costs nothing and keeps the build honest about what it exercises.
if (url.startsWith("file:")) {
  mkdirSync(dirname(url.slice("file:".length)), { recursive: true });
}

// The busy timeout applies to every pooled connection; a transaction holds one for its lifetime.
export const rawClient = createClient({ url, timeout: 5000 });

export const db = drizzle(rawClient, { schema });

type Db = typeof db;
export type Tx = Parameters<Parameters<Db["transaction"]>[0]>[0];
export type Executor = Db | Tx;

declare global {
  // On globalThis because Next bundles the routes and the instrumentation hook apart,
  // and a module-level variable would give each bundle its own queue.
  var cliphubTransactionTail: Promise<unknown> | undefined;
}

// Runs one write transaction at a time. Each holds its own pooled connection, and two
// of them open at once would make the second wait on the first while blocking the event loop.
export function transact<T>(work: (tx: Tx) => Promise<T>): Promise<T> {
  const previous = globalThis.cliphubTransactionTail ?? Promise.resolve();
  const run = previous.then(
    () => db.transaction(work),
    () => db.transaction(work),
  );
  globalThis.cliphubTransactionTail = run.catch(() => undefined);
  return run;
}
