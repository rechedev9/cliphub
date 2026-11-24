'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ChevronLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { ProduceBack } from '@/components/produce/use-produce-back';

/** The outline back button of a producer's action bar. */
export function ProduceBackButton({ back }: { back: ProduceBack }): ReactNode {
  if ('href' in back) {
    return (
      <Button variant="outline" size="sm" asChild>
        <Link href={back.href}><ChevronLeft aria-hidden />{back.label}</Link>
      </Button>
    );
  }
  return (
    <Button type="button" variant="outline" size="sm" loading={back.busy === true} onClick={back.onSelect} title={back.hint}>
      <ChevronLeft aria-hidden />{back.label}
    </Button>
  );
}
