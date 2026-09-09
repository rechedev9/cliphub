import { redirect } from "next/navigation";

import { auth } from "@/auth";

import { SiteFooter } from "./site-footer";

const STEPS = [
  {
    label: "01",
    title: "Sube tu demo",
    body: "Entras con Google o Discord y subes el archivo .dem de la partida. Si quieres, dejas una nota: quién eres en la partida, qué ronda te interesa.",
  },
  {
    label: "02",
    title: "La reviso a mano",
    body: "Cada petición pasa por mí antes de entrar en la cola. No es una granja de renders: es un ordenador grabando una partida cada vez.",
  },
  {
    label: "03",
    title: "Recibes el vídeo",
    body: "Cuando está montado aparece en tu panel con su enlace de descarga. Tienes 30 días para bajártelo.",
  },
];

export default async function Home() {
  const session = await auth();
  if (session?.user) redirect("/dashboard");

  return (
    <main>
      <section className="hero hero-split">
        <div>
          <p className="eyebrow">
            <span className="rec-dot" aria-hidden="true" />
            CS2 · demo → vídeo
          </p>
          <h1>
            Tus mejores rondas, <span className="accent">montadas a mano</span>.
          </h1>
          <p className="lede">
            Mándame la demo de tu partida de Counter-Strike 2 y te devuelvo un
            vídeo editado con tus jugadas. Gratis, y de una en una.
          </p>
          <div className="cta-row">
            <a className="button" href="/login">
              Enviar una demo
            </a>
            <p className="cta-note">Entrar con Google o Discord</p>
          </div>
        </div>

        <div className="readout" aria-hidden="true">
          <div className="readout-head">
            <span className="rec-dot" />
            en proceso
          </div>
          <div className="readout-row">
            <span>mirage.dem</span>
            <span className="value">182 MB</span>
          </div>
          <div className="readout-arrow">↓</div>
          <div className="readout-row">
            <span>grabando en el equipo</span>
            <span className="value">1 / 1</span>
          </div>
          <div className="readout-arrow">↓</div>
          <div className="readout-row is-done">
            <span>reel-01.mp4</span>
            <span className="value">listo</span>
          </div>
        </div>
      </section>

      <h2 className="section-title">Cómo funciona</h2>
      <ol className="steps">
        {STEPS.map((step) => (
          <li key={step.label} className="step">
            <span className="step-number">{step.label}</span>
            <h3>{step.title}</h3>
            <p>{step.body}</p>
          </li>
        ))}
      </ol>

      <h2 className="section-title">Antes de que envíes nada</h2>
      <div className="card">
        <p className="request-note">
          Esto lo llevo yo solo, con un único equipo que graba las partidas en
          el propio juego. Por eso hay cola, hay un límite de peticiones por
          persona, y puedo rechazar una demo sin más. Si tienes prisa, este no
          es tu sitio; si quieres un vídeo hecho con cuidado, adelante.
        </p>
      </div>

      <SiteFooter />
    </main>
  );
}
