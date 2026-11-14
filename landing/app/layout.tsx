import type { Metadata, Viewport } from "next";
import { IBM_Plex_Mono, IBM_Plex_Sans, Saira } from "next/font/google";
import "./globals.css";

const saira = Saira({
  variable: "--font-saira",
  subsets: ["latin"],
  weight: ["500", "600", "700", "800"],
  display: "swap",
});

const plexSans = IBM_Plex_Sans({
  variable: "--font-plex-sans",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  display: "swap",
});

const plexMono = IBM_Plex_Mono({
  variable: "--font-plex-mono",
  subsets: ["latin"],
  weight: ["400", "500"],
  display: "swap",
});

const TITLE = "ClipHub Studio: tus partidas de CS2, convertidas en vídeo";
const DESCRIPTION =
  "Convierte una demo de CS2 en un Short vertical o en la partida entera grabada desde un jugador, recorta tus streams y analiza tus partidas. Gratis y en tu PC con Windows.";
const OG_ALT = "ClipHub Studio: de la demo de CS2 al vídeo editado";

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
        url: "/opengraph-image?v=recorrido-1",
        width: 1200,
        height: 630,
        alt: OG_ALT,
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: TITLE,
    description: DESCRIPTION,
    images: ["/opengraph-image?v=recorrido-1"],
  },
};

export const viewport: Viewport = {
  themeColor: "#0b0e0d",
  colorScheme: "dark",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="es">
      <body className={`${saira.variable} ${plexSans.variable} ${plexMono.variable} font-sans antialiased`}>
        {children}
      </body>
    </html>
  );
}
