import type { Metadata } from "next";
import { redirect } from "next/navigation";

import { auth } from "@/auth";
import { Eyebrow } from "@/components/hud";
import { PageShell } from "@/components/page-shell";

import { SiteFooter } from "../site-footer";
import { LinkForm } from "./link-form";

export const metadata: Metadata = { title: "Vincular dispositivo" };

export default async function LinkPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const code = typeof params.code === "string" ? params.code : "";

  const session = await auth();
  if (!session?.user?.id) {
    const target = code === "" ? "/link" : `/link?code=${encodeURIComponent(code)}`;
    redirect(`/login?callbackUrl=${encodeURIComponent(target)}`);
  }

  return (
    <PageShell width="read">
      <section className="pt-8 pb-6">
        <Eyebrow>ClipHub</Eyebrow>
        <h1 className="mb-4.5 max-w-[20ch] text-[clamp(1.75rem,4vw,2.25rem)] leading-[1.15] font-semibold tracking-[-0.01em]">
          Conecta ClipHub Studio con tu cuenta
        </h1>
        <p className="max-w-[46ch] text-[1.125rem] leading-[1.6] text-fg-2">
          Así Studio puede enviar tus grabaciones a la nube de ClipHub y traerte los vídeos terminados.
        </p>
      </section>

      <LinkForm initialCode={code} accountLabel={session.user.name ?? session.user.email ?? "tu cuenta"} />
      <SiteFooter />
    </PageShell>
  );
}
