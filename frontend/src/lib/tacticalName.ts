import type { TacticalName } from '../api/types';

export const tacticalNameParts = [
  'organisation',
  'regionalAssociation',
  'localAssociation',
  'function',
  'number',
] as const satisfies readonly (keyof TacticalName)[];

/** Matches the server's limit per part. */
export const MAX_TACTICAL_NAME_PART_LENGTH = 32;

/** Trims all parts and drops the empty ones; null when none is left. */
export function cleanTacticalName(name: TacticalName): TacticalName | null {
  const out: TacticalName = {};
  for (const key of tacticalNameParts) {
    const v = name[key]?.trim();
    if (v) out[key] = v;
  }
  return Object.keys(out).length > 0 ? out : null;
}

/**
 * Formats the call sign as it is spoken, e.g. "Rotkreuz Musterstadt 12/83-1";
 * separators next to missing parts are left out. Empty for a missing name.
 */
export function formatTacticalName(name: TacticalName | null): string {
  if (!name) return '';
  const words = [name.organisation, name.regionalAssociation].filter(Boolean);
  const localFunction = [name.localAssociation, name.function].filter(Boolean).join('/');
  const code = [localFunction, name.number].filter(Boolean).join('-');
  return [...words, code].filter(Boolean).join(' ');
}
