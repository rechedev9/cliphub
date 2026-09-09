// Run after signing in for real at least once, to find the value to put in
// ADMIN_ACCOUNTS: `node scripts/whoami.mjs` from the portal/ directory.
import { createClient } from "@libsql/client";

const client = createClient({
  url: process.env.DATABASE_URL ?? "file:./data/portal.db",
});

const { rows } = await client.execute(`
  select u.email, u.name, a.provider, a.providerAccountId
  from user u
  join account a on a.userId = u.id
  order by u.email
`);

if (rows.length === 0) {
  console.log("No hay ninguna cuenta todavía. Inicia sesión una vez y vuelve a ejecutar esto.");
} else {
  console.log("Cuentas vinculadas (usa provider:providerAccountId en ADMIN_ACCOUNTS):\n");
  for (const row of rows) {
    console.log(`  ${row.email ?? row.name}  ->  ${row.provider}:${row.providerAccountId}`);
  }
}
