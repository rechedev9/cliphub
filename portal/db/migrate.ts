import { migrate } from "drizzle-orm/libsql/migrator";

import { db, rawClient } from "./client.ts";

// drizzle-kit is a dev dependency and needs the TypeScript schema, so the
// production image cannot shell out to it. The generated SQL in db/migrations
// is copied into the image instead and applied here on boot, which also means
// a fresh volume becomes a working database without a manual step.
export async function runMigrations(): Promise<void> {
  const migrationsFolder = process.env.MIGRATIONS_DIR ?? "./db/migrations";
  await migrate(db, { migrationsFolder });
  // WAL lets readers run beside the single writer; it persists in the database file.
  await rawClient.execute("PRAGMA journal_mode=WAL");
  await rawClient.execute("PRAGMA busy_timeout=5000");
}
