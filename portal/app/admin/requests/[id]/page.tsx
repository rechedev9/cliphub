import { asc, eq } from "drizzle-orm";
import { notFound } from "next/navigation";

import { db } from "@/db/client";
import { requestArtifacts, requests, users } from "@/db/schema";

import { StatusPill } from "../../../dashboard/status-pill";
import { DeliverButton, FailButton } from "./artifact-actions";

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
    <main>
      <header>
        <h1>Petición</h1>
        <a className="button secondary" href="/admin">
          Volver
        </a>
      </header>

      <div className="card">
        <StatusPill status={request.status} />
        <p>
          <strong>{request.userName ?? request.userEmail}</strong>
          {" — "}
          {request.demoOriginalName ?? "demo.dem"}
        </p>
        {request.note && <p className="request-note">“{request.note}”</p>}
        <p className="request-note">
          {new Date(request.createdAt).toLocaleString("es-ES")}
          {request.localJobId && ` · job local ${request.localJobId}`}
        </p>
        {request.failureReason && (
          <p className="error-text">{request.failureReason}</p>
        )}
      </div>

      <h2 style={{ fontSize: "1rem" }}>
        Vídeos candidatos ({artifacts.length})
      </h2>
      {artifacts.length === 0 && (
        <p className="card request-note">
          Todavía no ha llegado ningún render desde tu PC. Aparecerán aquí
          automáticamente cuando termines un render en el Studio.
        </p>
      )}

      {artifacts.map((artifact) => {
        const isCurrent = request.finalVideoPath === artifact.path;
        return (
          <div key={artifact.id} className="card">
            <p>
              <strong>{artifact.variant}</strong> · {artifact.name}{" "}
              <span className="request-note">
                ({formatSize(artifact.sizeBytes)})
              </span>
              {isCurrent && " · entregado"}
            </p>
            <video
              controls
              preload="metadata"
              style={{ width: "100%", borderRadius: "8px", marginBottom: "0.75rem" }}
              src={`/api/admin/requests/${id}/artifacts/${artifact.id}`}
            />
            <DeliverButton
              requestId={id}
              artifactId={artifact.id}
              isCurrent={isCurrent}
            />
          </div>
        );
      })}

      <FailButton requestId={id} />
    </main>
  );
}
