import type { Metadata } from "next";

import { SiteFooter } from "../site-footer";

export const metadata: Metadata = {
  title: "Condiciones del Servicio",
};

const CONTACT_EMAIL = process.env.CONTACT_EMAIL ?? "rsnoverwatch@gmail.com";

export default function TermsPage() {
  return (
    <main className="narrow doc">
      <h1>Condiciones del Servicio</h1>
      <p className="updated">Última actualización: 7 de septiembre de 2026</p>

      <h2>Qué es esto</h2>
      <p>
        ClipHub es un servicio gratuito y personal que convierte demos de
        Counter-Strike 2 en vídeos editados. Lo lleva una sola persona, con un
        solo ordenador, y cada vídeo se revisa y se produce a mano.
      </p>

      <h2>Sin garantías de entrega</h2>
      <p>
        Enviar una demo no da derecho a recibir un vídeo. Cada petición se
        revisa manualmente y puede ser rechazada, o no llegar a procesarse, sin
        necesidad de justificación. Tampoco hay plazos comprometidos: la cola
        depende del tiempo y la disponibilidad del operador.
      </p>
      <p>
        Existen límites por usuario en el número de peticiones simultáneas y
        diarias, para que el servicio siga siendo utilizable por todos.
      </p>

      <h2>Lo que subes</h2>
      <p>
        Solo debes subir demos de partidas en las que hayas participado o sobre
        las que tengas derechos. No subas archivos que no sean demos de CS2, ni
        contenido ilícito o que vulnere derechos de terceros. Las peticiones que
        incumplan esto se rechazan.
      </p>

      <h2>Conservación de los archivos</h2>
      <p>
        Los vídeos entregados se borran automáticamente a los 30 días. Descarga
        el tuyo dentro de ese plazo: no se conservan copias después.
      </p>

      <h2>Sin garantía y sin responsabilidad</h2>
      <p>
        El servicio se presta &laquo;tal cual&raquo;, sin garantía de ningún
        tipo. No se asume responsabilidad por interrupciones, pérdida de
        archivos, ni por el resultado del vídeo. El servicio puede cambiar,
        suspenderse o cerrarse en cualquier momento y sin aviso previo.
      </p>

      <h2>Contacto</h2>
      <p>
        Para cualquier duda sobre estas condiciones:{" "}
        <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.
      </p>

      <SiteFooter />
    </main>
  );
}
