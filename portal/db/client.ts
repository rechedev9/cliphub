import { createClient } from "@libsql/client";
import { drizzle } from "drizzle-orm/libsql";

import * as schema from "./schema.ts";

export const rawClient = createClient({
  url: process.env.DATABASE_URL ?? "file:./data/portal.db",
});

export const db = drizzle(rawClient, { schema });
