import { asc, desc, eq, inArray } from "drizzle-orm";

import { PageHeader, PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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

const SECTION_TITLE = "mt-8 mb-3 text-body-lg font-semibold first-of-type:mt-0";
const LIST = "flex flex-col gap-4";
const ITEM = "flex flex-col items-start gap-3";

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
    <PageShell>
      <PageHeader title="Panel de administración">
        <Button asChild variant="outline" size="sm">
          <a href="/dashboard">Volver</a>
        </Button>
      </PageHeader>

      <h2 className={SECTION_TITLE}>Pendientes de revisión ({pending.length})</h2>
      {pending.length === 0 && (
        <Card>
          <CardContent className="text-fg-2">No hay peticiones pendientes.</CardContent>
        </Card>
      )}
      <ul className={LIST}>
        {pending.map((request) => (
          <li key={request.id}>
            <Card>
              <CardContent className={ITEM}>
                <p>
                  <strong className="font-semibold">
                    {request.userName ?? request.userEmail}
                  </strong>
                  {" · "}
                  {request.demoOriginalName ?? "demo.dem"}
                </p>
                {request.note && <p className="text-fg-2">“{request.note}”</p>}
                <p className="text-fg-2">
                  {new Date(request.createdAt).toLocaleString("es-ES")}
                </p>
                <ApproveRejectButtons requestId={request.id} />
              </CardContent>
            </Card>
          </li>
        ))}
      </ul>

      <h2 className={SECTION_TITLE}>En curso ({inFlight.length})</h2>
      {inFlight.length === 0 && (
        <Card>
          <CardContent className="text-fg-2">Nada en curso ahora mismo.</CardContent>
        </Card>
      )}
      <ul className={LIST}>
        {inFlight.map((request) => (
          <li key={request.id}>
            <Card>
              <CardContent className={ITEM}>
                <StatusPill status={request.status} />
                <p>
                  <strong className="font-semibold">
                    {request.userName ?? request.userEmail}
                  </strong>
                  {" · "}
                  {request.demoOriginalName ?? "demo.dem"}
                </p>
                {request.note && <p className="text-fg-2">“{request.note}”</p>}
                <Button asChild variant="outline" size="sm">
                  <a href={`/admin/requests/${request.id}`}>Ver y entregar</a>
                </Button>
              </CardContent>
            </Card>
          </li>
        ))}
      </ul>
    </PageShell>
  );
}
