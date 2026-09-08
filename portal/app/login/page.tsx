import { redirect } from "next/navigation";

import { auth, signIn } from "@/auth";

import { SiteFooter } from "../site-footer";

export default async function LoginPage() {
  const session = await auth();
  if (session?.user) redirect("/dashboard");

  return (
    <main className="narrow">
      <section className="hero" style={{ borderBottom: "none", paddingBottom: "1.5rem" }}>
        <p className="eyebrow">
          <span className="rec-dot" aria-hidden="true" />
          ClipHub
        </p>
        <h1 style={{ fontSize: "clamp(1.75rem, 4vw, 2.25rem)" }}>Entra para enviar tu demo</h1>
        <p className="lede" style={{ marginBottom: 0 }}>
          Usamos tu cuenta solo para identificarte y enseñarte tus propias
          peticiones.
        </p>
      </section>

      <div className="card">
        <form
          action={async () => {
            "use server";
            await signIn("google", { redirectTo: "/dashboard" });
          }}
        >
          <button type="submit" style={{ width: "100%" }}>
            Continuar con Google
          </button>
        </form>
        <div style={{ height: "0.6rem" }} />
        <form
          action={async () => {
            "use server";
            await signIn("discord", { redirectTo: "/dashboard" });
          }}
        >
          <button type="submit" className="secondary" style={{ width: "100%" }}>
            Continuar con Discord
          </button>
        </form>
        <p className="request-note" style={{ marginTop: "1.25rem", marginBottom: 0 }}>
          Al entrar aceptas las <a href="/terms">condiciones del servicio</a> y
          la <a href="/privacy">política de privacidad</a>.
        </p>
      </div>

      <SiteFooter />
    </main>
  );
}
