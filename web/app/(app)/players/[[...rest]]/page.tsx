import { redirect } from 'next/navigation';
import { retiredHref } from '@/lib/nav';

type RetiredProps = {
  params: Promise<{ rest?: string[] }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

/** Retired door: the FACEIT follow page moved to Jugadores > FACEIT. Nested paths and the query follow it. */
export default async function RetiredPage({ params, searchParams }: RetiredProps): Promise<never> {
  const [{ rest }, search] = await Promise.all([params, searchParams]);
  redirect(retiredHref('/players', { rest, search }));
}
