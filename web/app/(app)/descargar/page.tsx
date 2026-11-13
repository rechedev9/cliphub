import type { ReactNode } from 'react';
import { Download } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { DocumentPage } from '@/components/records/document-page';

const DOWNLOAD_URL = 'https://github.com/rechedev9/cliphub/releases/latest';

/** La edición web analiza partidas; la creación de vídeo se hace en la app de Windows. */
export default function DownloadPage(): ReactNode {
  return (
    <DocumentPage
      title="Crea vídeos con ClipHub Studio"
      description="La versión web es para analizar partidas y ver el replay 2D."
    >
      <Card>
        <CardContent className="flex flex-col items-start gap-4">
          <p className="measure-read text-body-sm text-fg-2">
            Grabar demos a MP4, preparar Shorts, demos completas y clips de stream necesita la app gratuita para Windows.
          </p>
          <Button asChild variant="default" size="lg">
            <a href={DOWNLOAD_URL}>
              <Download aria-hidden />
              Descargar ClipHub Studio
            </a>
          </Button>
        </CardContent>
      </Card>
    </DocumentPage>
  );
}
