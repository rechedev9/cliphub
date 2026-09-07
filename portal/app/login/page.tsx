import { redirect } from "next/navigation";

import { auth, signIn } from "@/auth";

import { SiteFooter } from "../site-footer";

export default async function LoginPage() {
  const session = await auth();
  if (session?.user) redirect("/dashboard");

  return (
    <main>
      <h1>ClipHub Portal</h1>
      <p className="request-note">
        Envía tu demo de CS2 y recibe tu vídeo cuando esté listo.
      </p>
      <div className="card">
        <form
          action={async () => {
            "use server";
            await signIn("google", { redirectTo: "/dashboard" });
          }}
        >
          <button type="submit">Continuar con Google</button>
        </form>
        <div style={{ height: "0.6rem" }} />
        <form
          action={async () => {
            "use server";
            await signIn("discord", { redirectTo: "/dashboard" });
          }}
        >
          <button type="submit" className="secondary">
            Continuar con Discord
          </button>
        </form>
        <p className="request-note" style={{ marginTop: "1rem" }}>
          Al entrar aceptas las <a href="/terms">condiciones del servicio</a> y
          la <a href="/privacy">política de privacidad</a>.
        </p>
      </div>
      <SiteFooter />
    </main>
  );
}
