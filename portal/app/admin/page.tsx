import { asc, eq } from "drizzle-orm";

import { db } from "@/db/client";
import { requests, users } from "@/db/schema";

import { ApproveRejectButtons } from "./approve-reject-buttons";

export default async function AdminPage() {
  const pending = await db
    .select({
      id: requests.id,
      note: requests.note,
      demoOriginalName: requests.demoOriginalName,
      createdAt: requests.createdAt,
      userEmail: users.email,
      userName: users.name,
    })
    .from(requests)
    .innerJoin(users, eq(users.id, requests.userId))
    .where(eq(requests.status, "pending"))
    .orderBy(asc(requests.createdAt));

  return (
    <main>
      <header>
        <h1>Panel de administración</h1>
        <a className="button secondary" href="/dashboard">
          Volver
        </a>
      </header>

      <p className="request-note">
        {pending.length === 0
          ? "No hay peticiones pendientes."
          : `${pending.length} petición(es) pendiente(s) de revisión.`}
      </p>

      <ul className="requests">
        {pending.map((request) => (
          <li key={request.id} className="card">
            <p>
              <strong>{request.userName ?? request.userEmail}</strong>
              {" — "}
              {request.demoOriginalName ?? "demo.dem"}
            </p>
            {request.note && <p className="request-note">“{request.note}”</p>}
            <p className="request-note">
              {new Date(request.createdAt).toLocaleString("es-ES")}
            </p>
            <ApproveRejectButtons requestId={request.id} />
          </li>
        ))}
      </ul>
    </main>
  );
}
