import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

// Store tests import this file first: db/client.ts reads DATABASE_URL when it
// loads, so the throwaway database has to be chosen before that happens.
const dir = mkdtempSync(join(tmpdir(), "cliphub-portal-test-"));

process.env.DATABASE_URL = `file:${dir.replaceAll("\\", "/")}/portal.db`;
process.env.UPLOAD_DIR = join(dir, "uploads");
process.env.MIGRATIONS_DIR = fileURLToPath(new URL("../db/migrations", import.meta.url));
process.env.CLOUD_ACCESS_DEFAULT = "allowed";
process.env.CLOUD_KINDS = "short";
process.env.ADMIN_ACCOUNTS = "";
// The temp volume of a developer machine can be nearly full; the guard has its own test.
process.env.MIN_FREE_BYTES = "1";
process.env.MIN_FREE_BYTES_CLAIM = "1";

process.on("exit", () => {
  try {
    rmSync(dir, { recursive: true, force: true });
  } catch {
    // Windows keeps the database file locked until the process is gone.
  }
});

export const TEST_DIR = dir;
