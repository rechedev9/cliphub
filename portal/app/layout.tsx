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

export const metadata: Metadata = {
  title: { default: "ClipHub", template: "%s · ClipHub" },
  description:
    "Envía la demo de tu partida de CS2 y recibe un vídeo editado con tus mejores jugadas.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // The next/font variables must sit on <html> so the composed --font-sans and
  // --font-mono tokens in globals.css resolve at :root.
  return (
    <html lang="es" className={`${chakraPetch.variable} ${shareTechMono.variable}`}>
      <body>{children}</body>
    </html>
  );
}
