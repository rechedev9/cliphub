import { eq } from "drizzle-orm";

import { db } from "../db/client.ts";
import { accounts } from "../db/schema.ts";

export interface AdminAccountRef {
  provider: string;
  providerAccountId: string;
}

// Admins are identified by OAuth account (provider + providerAccountId), not
// email: Auth.js always populates providerAccountId from the OAuth sub/id
// regardless of provider config, whereas emailVerified is NOT populated by
// the default Google/Discord provider profile mapping (it stays null) and
// Discord emails aren't verified/stable identifiers at all. Configure via
// ADMIN_ACCOUNTS="google:1048...,discord:9382..." (comma-separated
// provider:providerAccountId pairs).
export function parseAdminAccounts(raw: string | undefined): AdminAccountRef[] {
  return (raw ?? "")
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean)
    .map((entry) => {
      const [provider, providerAccountId] = entry.split(":").map((s) => s.trim());
      return { provider: provider ?? "", providerAccountId: providerAccountId ?? "" };
    })
    .filter((ref) => ref.provider !== "" && ref.providerAccountId !== "");
}

export async function isAdminUser(userId: string): Promise<boolean> {
  const allow = parseAdminAccounts(process.env.ADMIN_ACCOUNTS);
  if (allow.length === 0) return false;

  const linked = await db
    .select({ provider: accounts.provider, providerAccountId: accounts.providerAccountId })
    .from(accounts)
    .where(eq(accounts.userId, userId));

  return linked.some((account) =>
    allow.some(
      (ref) =>
        ref.provider === account.provider &&
        ref.providerAccountId === account.providerAccountId,
    ),
  );
}
