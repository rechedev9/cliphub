import { DrizzleAdapter } from "@auth/drizzle-adapter";
import NextAuth from "next-auth";
import Discord from "next-auth/providers/discord";
import Google from "next-auth/providers/google";

import { db } from "./db/client";
import { accounts, sessions, users, verificationTokens } from "./db/schema";

export const { handlers, auth, signIn, signOut } = NextAuth({
  adapter: DrizzleAdapter(db, {
    usersTable: users,
    accountsTable: accounts,
    sessionsTable: sessions,
    verificationTokensTable: verificationTokens,
  }),
  session: { strategy: "database" },
  // Discord returns an `iss` parameter; without its issuer here Auth.js rejects the callback.
  providers: [Google, Discord({ issuer: "https://discord.com" })],
  pages: {
    signIn: "/login",
  },
});
