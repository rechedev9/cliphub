import { fileURLToPath } from "node:url";

// Imported first by dev-seed.ts: db/client.ts opens the database when it loads,
// so the refusal and the defaults have to be in place before that.
if (process.env.NODE_ENV === "production") {
  console.error("dev-seed crea credenciales de prueba y no se ejecuta en producción.");
  process.exit(1);
}

function portalPath(relative: string): string {
  return fileURLToPath(new URL(`../${relative}`, import.meta.url)).replaceAll("\\", "/");
}

// The same database `pnpm run dev` uses, whatever directory the script is started from.
process.env.DATABASE_URL ??= `file:${portalPath("data/portal.db")}`;
process.env.MIGRATIONS_DIR ??= portalPath("db/migrations");
