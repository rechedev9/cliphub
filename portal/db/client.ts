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

export const rawClient = createClient({ url });

export const db = drizzle(rawClient, { schema });
