import type { Metadata } from "next";

import { DocPage } from "@/components/doc-page";

export const metadata: Metadata = {
  title: "Condiciones del Servicio",
};

const CONTACT_EMAIL = process.env.CONTACT_EMAIL ?? "rsnoverwatch@gmail.com";

export default function TermsPage() {
  return (
    <DocPage title="Condiciones del Servicio" updated="Última actualización: 10 de octubre de 2026">

      <h2>Qué es esto</h2>
      <p>
        ClipHub es un servicio gratuito y personal que convierte demos de
        Counter-Strike 2 en vídeos. Lo lleva una sola persona. Con ClipHub
        Studio eliges, en cada vídeo, si se graba en tu PC o en la nube de
        ClipHub. Estas condiciones se refieren a la captura en la nube, que
        necesita una cuenta y que Studio esté vinculado a ella.
      </p>

      <h2>Acceso y sin garantías de entrega</h2>
      <p>
        El acceso a la nube se concede por cuenta y puede estar pendiente de
        aprobación, o ser suspendido, sin necesidad de justificación. Pedir un
        vídeo no da derecho a recibirlo: el trabajo espera en una cola que se
        reparte entre usuarios, puede fallar o no llegar a procesarse, y no hay
        plazos comprometidos. Si el equipo de grabación no está disponible, la
        cola se detiene.
      </p>
      <p>
        Existen límites por usuario en el número de trabajos en cola y en
        grabación, y en el espacio que ocupan tus demos, para que el servicio
        siga siendo utilizable por todos.
      </p>

      <h2>Lo que envías</h2>
      <p>
        Solo debes enviar demos de partidas en las que hayas participado o
        sobre las que tengas derechos. No envíes archivos que no sean demos de
        CS2, ni contenido ilícito o que vulnere derechos de terceros. Los
        trabajos que incumplan esto pueden cancelarse y la cuenta, suspenderse.
      </p>

      <h2>Conservación de los archivos</h2>
      <p>
        Tu demo se guarda mientras algún trabajo tuyo la necesite y se borra
        después; si el trabajo falla, se conserva como máximo 48 horas. El
        vídeo terminado se descarga con Studio o desde tu panel, y se borra de
        nuestros servidores 24 horas después de que Studio confirme que lo
        recibió, o a los 7 días de generarse si nunca lo confirmó. Descárgalo
        dentro de ese plazo: no se conservan copias después.
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
    </DocPage>
  );
}
