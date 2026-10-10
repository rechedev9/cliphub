"use client";

import { useCallback, useEffect, useState } from "react";

import { Notice, NoticeDetail } from "@/components/notice";
import { TextLink } from "@/components/text-link";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import type { LinkInfo } from "../components/api-types";
import { fetchJson } from "../components/fetch-json";
import { formatClock, normalizeUserCode } from "../components/format";
import { actionErrorText, loadErrorText } from "../components/labels";
import { parseLinkInfo } from "../components/parse";
import { postAction } from "../components/post-action";

type Phase =
  | { kind: "enter"; problem: string | null }
  | { kind: "looking" }
  | { kind: "found"; code: string; info: LinkInfo; typed: string; busy: boolean; problem: string | null }
  | { kind: "approved"; deviceName: string }
  | { kind: "denied" }
  | { kind: "expired" };

const UNKNOWN_CODE = "Ese código no existe, ya se usó o ha caducado. Comprueba que lo has escrito como aparece en Studio.";

const BODY = "flex flex-col items-start gap-4";
const TITLE = "text-title font-semibold";
// The code reads as it does in Studio: wide, mono, upper case. 16px or more, so phones do not zoom.
const CODE_INPUT = "h-14 text-center font-mono text-[1.5rem] tracking-[0.2em] uppercase md:text-[1.5rem]";

export interface LinkFormProps {
  initialCode: string;
  accountLabel: string;
}

export function LinkForm({ initialCode, accountLabel }: LinkFormProps) {
  const [input, setInput] = useState(initialCode);
  const [phase, setPhase] = useState<Phase>({ kind: initialCode === "" ? "enter" : "looking", problem: null });

  const lookup = useCallback(async (raw: string) => {
    const code = normalizeUserCode(raw);
    if (code === null) {
      setPhase({ kind: "enter", problem: "El código tiene 8 letras y números, por ejemplo K7QM-2XHD." });
      return;
    }
    setPhase({ kind: "looking" });
    const result = await fetchJson(`/api/link/${encodeURIComponent(code)}`, parseLinkInfo);
    if (!result.ok) {
      setPhase({ kind: "enter", problem: result.code === "not_found" ? UNKNOWN_CODE : loadErrorText(result.code) });
      return;
    }
    if (result.data.expiresAt <= Date.now()) {
      setPhase({ kind: "expired" });
      return;
    }
    // A code that arrived in a link from somewhere else is not left on screen to be copied.
    if (!result.data.sameNetwork) window.history.replaceState(null, "", "/link");
    setPhase({ kind: "found", code, info: result.data, typed: "", busy: false, problem: null });
  }, []);

  useEffect(() => {
    if (initialCode !== "") void lookup(initialCode);
  }, [initialCode, lookup]);

  async function answer(approve: boolean) {
    if (phase.kind !== "found") return;
    const current = phase;
    setPhase({ ...current, busy: true, problem: null });
    const typedCode = approve && !current.info.sameNetwork ? { typedCode: current.typed } : {};
    const result = await postAction({
      url: "/api/link/approve",
      body: { userCode: current.code, approve, ...typedCode },
    });
    if (result.ok) {
      setPhase(approve ? { kind: "approved", deviceName: current.info.deviceName } : { kind: "denied" });
    } else if (result.code === "not_found") {
      setPhase({ kind: "expired" });
    } else {
      setPhase({ ...current, busy: false, problem: actionErrorText(result.code) });
    }
  }

  const restart = () => {
    setInput("");
    setPhase({ kind: "enter", problem: null });
  };

  if (phase.kind === "approved") {
    return (
      <Card tone="accent" role="status">
        <CardContent className={BODY}>
          <h2 className={TITLE}>Dispositivo vinculado</h2>
          <p>
            <strong className="font-semibold wrap-anywhere">{phase.deviceName}</strong> ya puede grabar en la nube con
            tu cuenta. Vuelve a ClipHub Studio: se conectará solo en unos segundos. Puedes cerrar esta pestaña.
          </p>
          <p className="text-fg-2">
            Puedes desvincularlo cuando quieras desde <TextLink href="/dashboard">tu panel</TextLink>.
          </p>
        </CardContent>
      </Card>
    );
  }

  if (phase.kind === "denied") {
    return (
      <Card role="status">
        <CardContent className={BODY}>
          <h2 className={TITLE}>Vinculación rechazada</h2>
          <p>No se ha vinculado ningún dispositivo. Si alguien te ha pedido este código, no se lo des.</p>
          <Button type="button" variant="outline" onClick={restart}>
            Usar otro código
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (phase.kind === "expired") {
    return (
      <Card tone="warning" role="alert">
        <CardContent className={BODY}>
          <h2 className={TITLE}>El código ha caducado</h2>
          <p>Los códigos duran 10 minutos y solo sirven una vez. Pide uno nuevo en ClipHub Studio y vuelve aquí.</p>
          <Button type="button" variant="outline" onClick={restart}>
            Escribir un código nuevo
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (phase.kind === "found") {
    const current = phase;
    const elsewhere = !current.info.sameNetwork;
    // From another network the button only works once the code from Studio has been typed.
    const ready = !elsewhere || normalizeUserCode(current.typed) !== null;
    return (
      <Card tone={elsewhere ? "danger" : "neutral"}>
        <CardContent className={BODY}>
          <h2 className={TITLE}>¿Vincular este dispositivo?</h2>
          <p>
            El equipo <strong className="font-semibold wrap-anywhere">{current.info.deviceName}</strong> pide permiso
            para grabar en la nube con la cuenta <strong className="font-semibold wrap-anywhere">{accountLabel}</strong>
            .
          </p>
          {elsewhere ? (
            <>
              <Notice tone="danger" role="alert" className="w-full">
                <div className="min-w-0">
                  <strong className="font-semibold">Esta petición no viene de tu red.</strong>
                  <NoticeDetail>
                    Si has llegado aquí por un enlace o un código que te ha enviado otra persona, pulsa «No he sido
                    yo»: quien lo pidió podría usar la nube con tu cuenta y descargar tus vídeos.
                  </NoticeDetail>
                </div>
              </Notice>
              <div className="grid w-full gap-2">
                <Label htmlFor="link-typed" className="leading-snug">
                  Si lo has pedido tú, escribe el código que muestra ClipHub Studio en tu pantalla
                </Label>
                <Input
                  id="link-typed"
                  className={CODE_INPUT}
                  type="text"
                  value={current.typed}
                  autoComplete="off"
                  autoCapitalize="characters"
                  spellCheck={false}
                  maxLength={12}
                  placeholder="XXXX-XXXX"
                  onChange={(event) => setPhase({ ...current, typed: event.target.value, problem: null })}
                />
              </div>
              <p className="text-fg-2">La petición caduca a las {formatClock(current.info.expiresAt)}.</p>
            </>
          ) : (
            <p className="text-fg-2">
              Código <span className="font-mono text-fg-1">{current.code}</span> · caduca a las{" "}
              {formatClock(current.info.expiresAt)}. Acepta solo si acabas de pedirlo tú en ClipHub Studio.
            </p>
          )}
          {phase.problem !== null && (
            <p className="text-destructive" role="alert">
              {phase.problem}
            </p>
          )}
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" disabled={phase.busy || !ready} onClick={() => void answer(true)}>
              Vincular este dispositivo
            </Button>
            <Button type="button" variant="outline" disabled={phase.busy} onClick={() => void answer(false)}>
              No he sido yo
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void lookup(input);
      }}
    >
      <Card>
        <CardContent className={BODY}>
          <div className="grid w-full gap-2">
            <Label htmlFor="link-code">Código que muestra ClipHub Studio</Label>
            <Input
              id="link-code"
              className={CODE_INPUT}
              type="text"
              value={input}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              maxLength={12}
              placeholder="XXXX-XXXX"
              onChange={(event) => setInput(event.target.value)}
            />
          </div>
          {phase.kind === "enter" && phase.problem !== null && (
            <p className="text-destructive" role="alert">
              {phase.problem}
            </p>
          )}
          <Button type="submit" disabled={phase.kind === "looking" || input.trim() === ""}>
            {phase.kind === "looking" ? "Buscando" : "Continuar"}
          </Button>
        </CardContent>
      </Card>
    </form>
  );
}
