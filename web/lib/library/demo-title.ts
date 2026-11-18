/** The server's cap on a demo title (library.MaxDemoTitleRunes), counted in characters. */
export const MAX_DEMO_TITLE_LENGTH = 80;

type HeadingInput = { title?: string | undefined; teams: string; fallback: string };

/** A demo card's heading: the person's title, else "A vs B", else what the card showed before; under a title, "A vs B" moves to the second line. */
export function demoHeading({ title, teams, fallback }: HeadingInput): { heading: string; secondary: string } {
  if (title !== undefined && title !== '') return { heading: title, secondary: teams };
  return { heading: teams === '' ? fallback : teams, secondary: '' };
}

/** The draft cut to the cap; an input's maxLength counts UTF-16 units, so an emoji would count twice. */
export function clipDemoTitle(draft: string): string {
  const chars = [...draft];
  return chars.length > MAX_DEMO_TITLE_LENGTH ? chars.slice(0, MAX_DEMO_TITLE_LENGTH).join('') : draft;
}
