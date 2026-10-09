import type { Metadata } from "next";
import { Chakra_Petch, Share_Tech_Mono } from "next/font/google";

import "./globals.css";

// The same pair the Studio uses, so the portal reads as the same product.
const chakraPetch = Chakra_Petch({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-chakra-petch",
});

const shareTechMono = Share_Tech_Mono({
  subsets: ["latin"],
  weight: "400",
  variable: "--font-share-tech-mono",
});

const SITE_TITLE = "ClipHub · Tus mejores rondas de CS2, montadas a mano";
const SITE_DESCRIPTION =
  "Envía la demo de tu partida de CS2 y recibe un vídeo editado con tus mejores jugadas, grabado dentro del juego. Gratis.";

export const metadata: Metadata = {
  // AUTH_URL is the public origin in production; link previews need it absolute.
  metadataBase: new URL(process.env.AUTH_URL || "https://cliphub.gravityroom.app"),
  title: { default: SITE_TITLE, template: "%s · ClipHub" },
  description: SITE_DESCRIPTION,
  openGraph: {
    type: "website",
    siteName: "ClipHub",
    locale: "es_ES",
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
    url: "/",
  },
  twitter: {
    card: "summary_large_image",
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
  },
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // The next/font variables must sit on <html> so the composed --font-sans and
  // --font-mono tokens in globals.css resolve at :root.
  return (
    <html
      lang="es"
      className={`dark motion-safe:scroll-smooth ${chakraPetch.variable} ${shareTechMono.variable}`}
    >
      <body>{children}</body>
    </html>
  );
}
