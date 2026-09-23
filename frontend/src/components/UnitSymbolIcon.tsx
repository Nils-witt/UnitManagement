import { useMemo } from 'react';
import type { TacticalName, UnitSymbol } from '../api/types';
import { symbolDataUrl } from '../lib/unitSymbol';
import './UnitSymbolIcon.scss';

/** A unit's tactical symbol; keeps its space empty when there's none. */
export default function UnitSymbolIcon({
  symbol,
  size = 'small',
  tacticalName,
}: {
  symbol: UnitSymbol | null;
  size?: 'small' | 'large';
  tacticalName: TacticalName | null;
}) {
  const src = useMemo(() => symbolDataUrl(symbol, tacticalName), [symbol]);
  const className = `unit-symbol-icon unit-symbol-icon--${size}`;
  if (!src) return <span className={className} aria-hidden />;
  return <img className={className} src={src} alt="" />;
}
