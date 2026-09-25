export function fmtDate(iso: string, locale?: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? iso
    : d.toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' });
}

/** The API reports speed in m/s; the UI shows and takes km/h. */
export const KMH_PER_MS = 3.6;

/** Speed in m/s as km/h with one decimal. */
export function fmtSpeed(metersPerSecond: number): string {
  return (metersPerSecond * KMH_PER_MS).toFixed(1);
}
