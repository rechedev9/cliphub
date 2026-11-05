import type { Metadata, Viewport } from "next";
import { Chakra_Petch, Permanent_Marker, Share_Tech_Mono } from "next/font/google";
import "./globals.css";

const chakraPetch = Chakra_Petch({
  variable: "--font-chakra-petch",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

const shareTechMono = Share_Tech_Mono({
  variable: "--font-share-tech-mono",
  subsets: ["latin"],
  weight: "400",
  display: "swap",
});

const permanentMarker = Permanent_Marker({
  variable: "--font-permanent-marker",
  subsets: ["latin"],
  weight: "400",
  display: "swap",
});

const TITLE = "ClipHub Studio: tus partidas de CS2, convertidas en vídeo";
const DESCRIPTION =
  "Convierte una demo de CS2 en un Short vertical o en la partida entera grabada desde un jugador, recorta tus streams y analiza tus partidas. Gratis y en tu PC con Windows.";

// Runs before first paint so the page never flashes the wrong layout. WebGL that only works with
// failIfMajorPerformanceCaveat off is software rendering, so that machine counts as weak.
const MODE_SCRIPT = `(function(){var d=document.documentElement;try{var q=new URLSearchParams(location.search),s=null;try{s=localStorage.getItem('ciber-lite')}catch(e){}var gl=false,fast=false;try{fast=!!document.createElement('canvas').getContext('webgl2',{failIfMajorPerformanceCaveat:true});gl=fast||!!document.createElement('canvas').getContext('webgl2')}catch(e){}d.dataset.webgl=gl?'1':'0';var n=navigator,weak=!fast||(n.deviceMemory&&n.deviceMemory<3)||(n.connection&&n.connection.saveData)||(n.hardwareConcurrency&&n.hardwareConcurrency<=2);var want=q.get('3d')==='1'||(q.get('lite')!=='1'&&s!=='1'&&(s==='0'||!weak));d.dataset.ciber=gl&&want?'3d':'lite'}catch(e){d.dataset.ciber='lite'}})();`;

export const metadata: Metadata = {
  metadataBase: new URL("https://cliphub.rechedev.cloud"),
  title: TITLE,
  description: DESCRIPTION,
  applicationName: "ClipHub Studio",
  keywords: ["CS2", "Counter-Strike 2", "demo a vídeo", "Shorts", "HLAE", "jugadas", "análisis de demos"],
  openGraph: {
    type: "website",
    locale: "es_ES",
    siteName: "ClipHub Studio",
    title: TITLE,
    description: DESCRIPTION,
    url: "/",
    images: [
      {
        url: "/opengraph-image?v=ciber-1",
        width: 1200,
        height: 630,
        alt: "Un puesto de ciber con ClipHub Studio en el monitor",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: TITLE,
    description: DESCRIPTION,
    images: ["/opengraph-image?v=ciber-1"],
  },
};

export const viewport: Viewport = {
  themeColor: "#0a0d11",
  colorScheme: "dark light",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="es" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: MODE_SCRIPT }} />
      </head>
      <body className={`${chakraPetch.variable} ${shareTechMono.variable} ${permanentMarker.variable} font-display antialiased`}>{children}</body>
    </html>
  );
}
