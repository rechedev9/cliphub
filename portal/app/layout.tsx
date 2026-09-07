import type { Metadata } from "next";

import "./globals.css";

export const metadata: Metadata = {
  title: "ClipHub Portal",
  description: "Envía tu demo de CS2 y recibe tu vídeo cuando esté listo.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="es">
      <body>{children}</body>
    </html>
  );
}
