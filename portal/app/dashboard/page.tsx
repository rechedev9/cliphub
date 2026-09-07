import { desc, eq } from "drizzle-orm";
import { redirect } from "next/navigation";

import { auth, signOut } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";

import { NewRequestForm } from "./new-request-form";
import { StatusPill } from "./status-pill";

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  const own = await db
    .select()
    .from(requests)
    .where(eq(requests.userId, session.user.id))
    .orderBy(desc(requests.createdAt));

  return (
    <main>
      <header>
        <h1>ClipHub Portal</h1>
        <form
          action={async () => {
            "use server";
            await signOut({ redirectTo: "/login" });
          }}
        >
          <button type="submit" className="secondary">
            Cerrar sesión
          </button>
        </form>
      </header>

      <NewRequestForm />

      <ul className="requests">
        {own.length === 0 && (
          <li className="card request-note">
            Todavía no has enviado ninguna demo.
          </li>
        )}
        {own.map((request) => (
          <li key={request.id} className="card">
            <StatusPill status={request.status} />
            {request.demoOriginalName && (
              <p className="request-note">{request.demoOriginalName}</p>
            )}
            {request.note && <p className="request-note">“{request.note}”</p>}
            {request.status === "failed" && request.failureReason && (
              <p className="error-text">{request.failureReason}</p>
            )}
            {request.status === "done" && request.finalVideoPath && (
              <p>
                <a
                  className="button"
                  href={`/api/requests/${request.id}/download`}
                >
                  Descargar vídeo
                </a>
              </p>
            )}
          </li>
        ))}
      </ul>
    </main>
  );
}
