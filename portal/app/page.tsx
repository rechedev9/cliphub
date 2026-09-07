import { redirect } from "next/navigation";

import { auth } from "@/auth";

import { SiteFooter } from "./site-footer";

export default async function Home() {
  const session = await auth();
  if (session?.user) redirect("/dashboard");

  return (
    <main>
      <h1>ClipHub</h1>
      <p className="lede">
        Envía la demo de tu partida de CS2 y recibe un vídeo editado con tus
        mejores jugadas.
      </p>

      <div className="card">
        <h2>Cómo funciona</h2>
        <ol>
          <li>
            Inicias sesión con Google o Discord y subes el archivo{" "}
            <code>.dem</code> de tu partida.
          </li>
          <li>
            Revisamos tu petición a mano y la ponemos en cola. No es
            instantáneo: cada vídeo se graba y se edita uno a uno.
          </li>
          <li>
            Cuando está listo lo verás en tu panel, con un enlace de descarga.
          </li>
        </ol>
        <p className="request-note">
          Es un servicio gratuito y artesanal, llevado por una sola persona. Hay
          un límite de peticiones por usuario y las demos se procesan por orden.
        </p>
        <p>
          <a className="button" href="/login">
            Entrar y enviar una demo
          </a>
        </p>
      </div>

      <SiteFooter />
    </main>
  );
}
