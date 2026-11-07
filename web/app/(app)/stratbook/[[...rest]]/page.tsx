import { redirect } from 'next/navigation';
import { retiredHref } from '@/lib/nav';

type RetiredProps = {
  params: Promise<{ rest?: string[] }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

/** Retired door: the Stratbook is the Jugadas tab of the Libreta. A play's page and the query follow it. */
export default async function RetiredPage({ params, searchParams }: RetiredProps): Promise<never> {
  const [{ rest }, search] = await Promise.all([params, searchParams]);
  redirect(retiredHref('/stratbook', { rest, search }));
}
