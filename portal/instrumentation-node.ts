import { runMigrations } from "./db/migrate.ts";
import { startRetentionSweeper } from "./lib/retention.ts";

// Migrations first: the sweeper and every route assume the tables exist, and
// a fresh deployment starts with an empty volume.
await runMigrations();
startRetentionSweeper();
