// Date labels of the EDGE-style lists: the group heading after the slash and the day on a map thumbnail.

const HEADING = new Intl.DateTimeFormat('es-ES', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' });
const DAY_MONTH = new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: 'short' });

function plain(text: string): string {
  return text.replaceAll('.', '').trim();
}

/** "lun 9 nov 2026" for a YYYY-MM-DD day; the raw day when it does not parse. */
export function slashDayLabel(day: string): string {
  const at = new Date(`${day}T12:00:00`);
  if (Number.isNaN(at.getTime())) return day;
  const parts = new Map(HEADING.formatToParts(at).map((part) => [part.type, plain(part.value)]));
  return [parts.get('weekday'), parts.get('day'), parts.get('month'), parts.get('year')].filter((part) => part !== undefined && part !== '').join(' ');
}

/** "09 nov" for an ISO date; an empty string when it does not parse. */
export function dayMonthLabel(iso: string): string {
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? '' : plain(DAY_MONTH.format(at));
}
