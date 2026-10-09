import { desc, eq } from "drizzle-orm";
import { redirect } from "next/navigation";

import { auth, signOut } from "@/auth";
import { PageHeader, PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isAdminUser } from "@/lib/admin";

import { NewRequestForm } from "./new-request-form";
import { StatusPill } from "./status-pill";

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  const [own, admin] = await Promise.all([
    db
      .select()
      .from(requests)
      .where(eq(requests.userId, session.user.id))
      .orderBy(desc(requests.createdAt)),
    isAdminUser(session.user.id),
  ]);

  return (
    <PageShell>
      <PageHeader title="ClipHub Portal">
        {admin && (
          <Button asChild variant="outline" size="sm">
            <a href="/admin">Admin</a>
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

      <NewRequestForm />

      <ul className="mt-4 flex flex-col gap-4">
        {own.length === 0 && (
          <li>
            <Card>
              <CardContent className="text-fg-2">
                Todavía no has enviado ninguna demo.
              </CardContent>
            </Card>
          </li>
        )}
        {own.map((request) => (
          <li key={request.id}>
            <Card>
              <CardContent className="flex flex-col items-start gap-3">
                <StatusPill status={request.status} />
                {request.demoOriginalName && (
                  <p className="text-fg-2">{request.demoOriginalName}</p>
                )}
                {request.note && <p className="text-fg-2">“{request.note}”</p>}
                {(request.status === "failed" || request.status === "rejected") &&
                  request.failureReason && (
                    <p className="text-destructive">{request.failureReason}</p>
                  )}
                {request.status === "done" &&
                  (request.finalVideoPath ? (
                    <Button asChild>
                      <a href={`/api/requests/${request.id}/download`}>Descargar vídeo</a>
                    </Button>
                  ) : (
                    <p className="max-w-[68ch] text-fg-2">
                      El vídeo ya no está disponible: los archivos entregados se
                      borran pasado el plazo de conservación.
                    </p>
                  ))}
              </CardContent>
            </Card>
          </li>
        ))}
      </ul>
    </PageShell>
  );
}
