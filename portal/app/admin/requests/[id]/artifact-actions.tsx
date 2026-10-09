"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

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
      <Button type="button" disabled={busy} onClick={deliver}>
        {isCurrent ? "Volver a entregar este" : "Entregar este vídeo"}
      </Button>
      {error && (
        <p className="text-destructive" role="alert">
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
    <Card>
      <CardContent className="flex flex-col gap-3">
        <Label htmlFor="fail-reason">Marcar como fallida</Label>
        <Textarea
          id="fail-reason"
          placeholder="Motivo que verá el usuario (opcional)"
          maxLength={500}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
        <Button
          type="button"
          variant="outline"
          className="self-start"
          disabled={busy}
          onClick={fail}
        >
          Marcar como fallida
        </Button>
        {error && (
          <p className="text-destructive" role="alert">
            {error}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
