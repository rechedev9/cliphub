import { asc, eq } from "drizzle-orm";
import { notFound } from "next/navigation";

import { PageHeader, PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { db } from "@/db/client";
import { requestArtifacts, requests, users } from "@/db/schema";

import { StatusPill } from "../../../dashboard/status-pill";
import { DeliverButton, FailButton } from "./artifact-actions";

const ITEM = "flex flex-col items-start gap-3";

function formatSize(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb.toFixed(1)} MB`;
}

export default async function AdminRequestPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const [request] = await db
    .select({
      id: requests.id,
      status: requests.status,
      note: requests.note,
      demoOriginalName: requests.demoOriginalName,
      localJobId: requests.localJobId,
      finalVideoPath: requests.finalVideoPath,
      failureReason: requests.failureReason,
      createdAt: requests.createdAt,
      userEmail: users.email,
      userName: users.name,
    })
    .from(requests)
    .innerJoin(users, eq(users.id, requests.userId))
    .where(eq(requests.id, id));

  if (!request) notFound();

  const artifacts = await db
    .select()
    .from(requestArtifacts)
    .where(eq(requestArtifacts.requestId, id))
    .orderBy(asc(requestArtifacts.uploadedAt));

  return (
    <PageShell>
      <PageHeader title="Petición">
        <Button asChild variant="outline" size="sm">
          <a href="/admin">Volver</a>
        </Button>
      </PageHeader>

      <Card>
        <CardContent className={ITEM}>
          <StatusPill status={request.status} />
          <p>
            <strong className="font-semibold">{request.userName ?? request.userEmail}</strong>
            {" · "}
            {request.demoOriginalName ?? "demo.dem"}
          </p>
          {request.note && <p className="text-fg-2">“{request.note}”</p>}
          <p className="text-fg-2">
            {new Date(request.createdAt).toLocaleString("es-ES")}
            {request.localJobId && ` · job local ${request.localJobId}`}
          </p>
          {request.failureReason && (
            <p className="text-destructive">{request.failureReason}</p>
          )}
        </CardContent>
      </Card>

      <h2 className="mt-8 mb-3 text-body-lg font-semibold">
        Vídeos candidatos ({artifacts.length})
      </h2>
      <div className="flex flex-col gap-4">
        {artifacts.length === 0 && (
          <Card>
            <CardContent className="max-w-[68ch] text-fg-2">
              Todavía no ha llegado ningún render desde tu PC. Aparecerán aquí
              automáticamente cuando termines un render en el Studio.
            </CardContent>
          </Card>
        )}

        {artifacts.map((artifact) => {
          const isCurrent = request.finalVideoPath === artifact.path;
          return (
            <Card key={artifact.id}>
              <CardContent className={ITEM}>
                <p>
                  <strong className="font-semibold">{artifact.variant}</strong> · {artifact.name}{" "}
                  <span className="text-fg-2">({formatSize(artifact.sizeBytes)})</span>
                  {isCurrent && " · entregado"}
                </p>
                <video
                  className="w-full rounded-md border border-border bg-surface-0"
                  controls
                  preload="metadata"
                  src={`/api/admin/requests/${id}/artifacts/${artifact.id}`}
                />
                <DeliverButton
                  requestId={id}
                  artifactId={artifact.id}
                  isCurrent={isCurrent}
                />
              </CardContent>
            </Card>
          );
        })}

        <FailButton requestId={id} />
      </div>
    </PageShell>
  );
}
