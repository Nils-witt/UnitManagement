export function fmtDate(iso: string, locale?: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? iso
    : d.toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' });
}
