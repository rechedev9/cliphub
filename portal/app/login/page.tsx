import { redirect } from "next/navigation";

import { auth, signIn } from "@/auth";
import { Eyebrow } from "@/components/hud";
import { PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

import { safeCallbackPath } from "../components/format";
import { SiteFooter } from "../site-footer";

const INLINE_LINK =
  "text-primary underline decoration-primary/35 underline-offset-[0.2em] hover:decoration-current";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { callbackUrl } = await searchParams;
  // Only a same-site path is honoured, so a crafted link cannot send a signed-in user elsewhere.
  const redirectTo = safeCallbackPath(typeof callbackUrl === "string" ? callbackUrl : undefined) ?? "/dashboard";
  const linking = redirectTo.startsWith("/link");

  const session = await auth();
  if (session?.user) redirect(redirectTo);

  return (
    <PageShell width="read">
      <section className="pt-8 pb-6">
        <Eyebrow>ClipHub</Eyebrow>
        <h1 className="mb-4.5 max-w-[20ch] text-[clamp(1.75rem,4vw,2.25rem)] leading-[1.15] font-semibold tracking-[-0.01em]">
          {linking ? "Entra para vincular ClipHub Studio" : "Entra en ClipHub"}
        </h1>
        <p className="max-w-[46ch] text-[1.125rem] leading-[1.6] text-fg-2">
          Usamos tu cuenta solo para identificarte y enseñarte tus propios
          trabajos.
        </p>
      </section>

      <Card>
        <CardContent className="flex flex-col gap-2.5">
          <form
            action={async () => {
              "use server";
              await signIn("google", { redirectTo });
            }}
          >
            <Button type="submit" className="w-full">
              Continuar con Google
            </Button>
          </form>
          <form
            action={async () => {
              "use server";
              await signIn("discord", { redirectTo });
            }}
          >
            <Button type="submit" variant="outline" className="w-full">
              Continuar con Discord
            </Button>
          </form>
          <p className="mt-2.5 text-fg-2">
            Al entrar aceptas las{" "}
            <a className={INLINE_LINK} href="/terms">
              condiciones del servicio
            </a>{" "}
            y la{" "}
            <a className={INLINE_LINK} href="/privacy">
              política de privacidad
            </a>
            .
          </p>
        </CardContent>
      </Card>

      <SiteFooter />
    </PageShell>
  );
}
