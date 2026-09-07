"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export function DeliverButton({
  requestId,
  artifactId,
  isCurrent,
}: {
  requestId: string;
  artifactId: string;
  isCurrent: boolean;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  async function deliver() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/requests/${requestId}/complete`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ artifactId }),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? "No se pudo entregar este vídeo.");
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <button type="button" disabled={busy} onClick={deliver}>
        {isCurrent ? "Volver a entregar este" : "Entregar este vídeo"}
      </button>
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
    </>
  );
}

export function FailButton({ requestId }: { requestId: string }) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  async function fail() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/requests/${requestId}/fail`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason: reason.trim() || undefined }),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? "No se pudo marcar como fallida.");
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="card">
      <label htmlFor="fail-reason">Marcar como fallida</label>
      <textarea
        id="fail-reason"
        placeholder="Motivo que verá el usuario (opcional)"
        maxLength={500}
        value={reason}
        onChange={(event) => setReason(event.target.value)}
      />
      <button type="button" className="secondary" disabled={busy} onClick={fail}>
        Marcar como fallida
      </button>
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
