import { StudioPageHeader } from '@/components/studio/page-header';
import { TacticalDemoPicker } from '@/components/tactical/tactical-demo-picker';
import { navSection } from '@/lib/nav';

const NAV = navSection('/tactical');

export const metadata = {
  title: 'Táctica — FragForge Studio',
};

/**
 * Entry point of the tactical workspace: pick the parsed demo to study. The
 * scan is per demo and costs a full parse, so this screen only lists and routes;
 * nothing here starts work.
 */
export default function TacticalIndexPage() {
  return (
    <div className="flex flex-col gap-8 sm:gap-10">
      <StudioPageHeader
        number={Number(NAV.number)}
        label={NAV.label.toUpperCase()}
        title="ANÁLISIS TÁCTICO"
        description="Rondas clasificadas, repetición 2D y tendencias, derivadas solo de la demo. Elige una partida para abrir su análisis."
      />
      <TacticalDemoPicker />
    </div>
  );
}
