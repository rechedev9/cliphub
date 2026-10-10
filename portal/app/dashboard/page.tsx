import { redirect } from "next/navigation";

import { auth, signOut } from "@/auth";
import { PageHeader, PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { isAdminUser } from "@/lib/admin";

import { SiteFooter } from "../site-footer";
import { DevicesView } from "./devices-view";
import { JobsView } from "./jobs-view";

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  const admin = await isAdminUser(session.user.id);

  return (
    <PageShell>
      <PageHeader title="ClipHub">
        {admin && (
          <Button asChild variant="outline" size="sm">
            <a href="/admin">Panel de control</a>
          </Button>
        )}
        <form
          action={async () => {
            "use server";
            await signOut({ redirectTo: "/login" });
          }}
        >
          <Button type="submit" variant="outline" size="sm">
            Cerrar sesión
          </Button>
        </form>
      </PageHeader>

      <JobsView />
      <DevicesView />
      <SiteFooter />
    </PageShell>
  );
}
