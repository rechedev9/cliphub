import type { Metadata } from 'next';
import { notFound } from 'next/navigation';

export const metadata: Metadata = { title: 'Página no encontrada' };

export default function MissingRoute(): never {
  notFound();
}
