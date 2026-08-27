import type { ReactNode } from 'react';
import { Lock } from 'lucide-react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { GuideStage } from '@/components/onboarding/guide-stage';
import { ShareCodeDoor } from '@/components/onboarding/share-code-door';
import { RecentSteamMatches } from '@/components/onboarding/recent-matches';
import { isHostedWebMode } from '@/lib/hosted-mode';

/** Inicio (`00`): first-run screen inside the app shell, not beside it. */
export default function OnboardingPage(): ReactNode {
  const hosted = isHostedWebMode();
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="EMPIEZA AQUÍ"
        description="ClipHub convierte una demo o un stream en un vídeo vertical listo para publicar, y lo hace entero en este PC. Elige por dónde entras."
      />

      <GuideStage />

      <div className="flex max-w-3xl flex-col gap-6">
        <ShareCodeDoor />
        <RecentSteamMatches />
      </div>

      <ul className="flex flex-wrap gap-x-6 gap-y-2 font-mono text-meta uppercase tracking-wider text-fg-3">
        <li className="inline-flex items-center gap-2">
          <Lock aria-hidden className="size-3.5 text-success" />
          El .dem no sale de tu PC
        </li>
        <li className="inline-flex items-center gap-2">
          <Lock aria-hidden className="size-3.5 text-success" />
          {hosted ? 'Cuenta y dispositivo protegidos' : 'Sin login'}
        </li>
      </ul>
    </div>
  );
}
