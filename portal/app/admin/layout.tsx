import type { Metadata } from "next";
import { redirect } from "next/navigation";

import { auth } from "@/auth";
import { PageHeader, PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { isAdminUser } from "@/lib/admin";

import { AdminNav } from "./admin-nav";

export const metadata: Metadata = { title: "Panel de control" };

export default async function AdminLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");
  if (!(await isAdminUser(session.user.id))) redirect("/dashboard");

  return (
    <PageShell width="panel" className="pt-8">
      {/* The navigation carries the rule under the header, so the header drops its own. */}
      <PageHeader title="Panel de control" className="mb-3 border-b-0 pb-0">
        <Button asChild variant="outline" size="sm">
          <a href="/dashboard">Volver a mi panel</a>
        </Button>
      </PageHeader>
      <AdminNav />
      {children}
    </PageShell>
  );
}
