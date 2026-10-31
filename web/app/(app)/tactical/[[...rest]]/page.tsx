import { redirect } from 'next/navigation';
import { retiredHref } from '@/lib/nav';

type RetiredProps = {
  params: Promise<{ rest?: string[] }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

/** Retired door: Táctica is now the Partidas library; a demo's workspace is its partida. Nested paths and the query follow it. */
export default async function RetiredPage({ params, searchParams }: RetiredProps): Promise<never> {
  const [{ rest }, search] = await Promise.all([params, searchParams]);
  redirect(retiredHref('/tactical', { rest, search }));
}
