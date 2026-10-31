export const DENSITY_STORAGE_KEY = 'cliphub-density';

export type Density = 'comfortable' | 'compact';

export const DENSITY_OPTIONS: ReadonlyArray<{ value: Density; label: string; hint: string }> = [
  { value: 'comfortable', label: 'Cómoda', hint: 'Más aire entre filas y controles.' },
  { value: 'compact', label: 'Compacta', hint: 'Más filas y datos por pantalla.' },
];

export function parseDensity(raw: string | null | undefined): Density {
  return raw === 'compact' ? 'compact' : 'comfortable';
}

/** Runs before the first paint so a compact window never flashes the comfortable layout. */
export const DENSITY_BOOT_SCRIPT = `try{if(localStorage.getItem('${DENSITY_STORAGE_KEY}')==='compact')document.documentElement.dataset.density='compact'}catch(e){}`;
