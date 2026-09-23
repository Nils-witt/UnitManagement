import {
  erzeugeTaktischesZeichen,
  grundzeichen,
  type ComponentType,
  type GrundzeichenId,
} from '@taktische-zeichen/core';
import type { UnitSymbol } from '../api/types';

/** The components that can be added on top of a Grundzeichen. */
export type SymbolComponent = Exclude<keyof UnitSymbol, 'grundzeichen'>;

const components: SymbolComponent[] = [
  'organisation',
  'fachaufgabe',
  'einheit',
  'verwaltungsstufe',
  'funktion',
  'symbol',
];

/** Drops the components the Grundzeichen can't show; null without a Grundzeichen. */
export function cleanSymbol(symbol: UnitSymbol): UnitSymbol | null {
  const g = symbol.grundzeichen;
  if (!g) return null;
  const out: UnitSymbol = { grundzeichen: g };
  for (const key of components) {
    if (symbol[key] && acceptsComponent(g, key)) Object.assign(out, { [key]: symbol[key] });
  }
  return out;
}

/** Whether the Grundzeichen can show the component; unknown IDs accept nothing. */
export function acceptsComponent(id: GrundzeichenId | undefined, component: ComponentType) {
  const g = grundzeichen.find((x) => x.id === id);
  return g != null && (g.accepts == null || g.accepts.includes(component));
}

/**
 * Renders the symbol as an SVG data URL. Null for a missing symbol or one the
 * library can't render, e.g. an ID it no longer knows.
 */
export function symbolDataUrl(symbol: UnitSymbol | null): string | null {
  if (!symbol || (!symbol.grundzeichen && !symbol.symbol)) return null;
  try {
    return erzeugeTaktischesZeichen(symbol).dataUrl;
  } catch {
    return null;
  }
}
