export type SortDirection = 'asc' | 'desc';

/** Case-insensitive, and "Object 2" sorts before "Object 10". */
export const naturalCollator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

/** Sorts by `primary` (text compares naturally, numbers numerically), breaking ties by `tiebreak` ascending. */
export function sortBy<T>(
  items: T[],
  direction: SortDirection,
  primary: (item: T) => string | number,
  tiebreak: (item: T) => string,
): T[] {
  const sign = direction === 'asc' ? 1 : -1;
  return [...items].sort((a, b) => {
    const pa = primary(a);
    const pb = primary(b);
    const diff =
      typeof pa === 'string' && typeof pb === 'string'
        ? naturalCollator.compare(pa, pb)
        : Number(pa) - Number(pb);
    // NaN (e.g. an unparsable date) counts as a tie.
    return (diff || 0) * sign || naturalCollator.compare(tiebreak(a), tiebreak(b));
  });
}
