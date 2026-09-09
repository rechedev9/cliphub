import { asc, desc, eq, inArray } from "drizzle-orm";

import { db } from "@/db/client";
import { requests, users } from "@/db/schema";

import { StatusPill } from "../dashboard/status-pill";
import { ApproveRejectButtons } from "./approve-reject-buttons";

const REQUEST_FIELDS = {
  id: requests.id,
  status: requests.status,
  note: requests.note,
  demoOriginalName: requests.demoOriginalName,
  createdAt: requests.createdAt,
  userEmail: users.email,
  userName: users.name,
};

export default async function AdminPage() {
  const [pending, inFlight] = await Promise.all([
    db
      .select(REQUEST_FIELDS)
      .from(requests)
      .innerJoin(users, eq(users.id, requests.userId))
      .where(eq(requests.status, "pending"))
      .orderBy(asc(requests.createdAt)),
    db
      .select(REQUEST_FIELDS)
      .from(requests)
      .innerJoin(users, eq(users.id, requests.userId))
      .where(inArray(requests.status, ["approved", "processing"]))
      .orderBy(desc(requests.createdAt)),
  ]);

  return (
    <main>
      <header>
        <h1>Panel de administración</h1>
        <a className="button secondary" href="/dashboard">
          Volver
        </a>
      </header>

      <h2 style={{ fontSize: "1rem" }}>
        Pendientes de revisión ({pending.length})
      </h2>
      {pending.length === 0 && (
        <p className="card request-note">No hay peticiones pendientes.</p>
      )}
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

      <h2 style={{ fontSize: "1rem" }}>En curso ({inFlight.length})</h2>
      {inFlight.length === 0 && (
        <p className="card request-note">Nada en curso ahora mismo.</p>
      )}
      <ul className="requests">
        {inFlight.map((request) => (
          <li key={request.id} className="card">
            <StatusPill status={request.status} />
            <p>
              <strong>{request.userName ?? request.userEmail}</strong>
              {" — "}
              {request.demoOriginalName ?? "demo.dem"}
            </p>
            {request.note && <p className="request-note">“{request.note}”</p>}
            <a className="button secondary" href={`/admin/requests/${request.id}`}>
              Ver y entregar
            </a>
          </li>
        ))}
      </ul>
    </main>
  );
}
