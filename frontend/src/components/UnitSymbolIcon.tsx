import { useMemo } from 'react';
import type { UnitSymbol } from '../api/types';
import { symbolDataUrl } from '../lib/unitSymbol';
import './UnitSymbolIcon.scss';

/** A unit's tactical symbol; keeps its space empty when there's none. */
export default function UnitSymbolIcon({
  symbol,
  size = 'small',
}: {
  symbol: UnitSymbol | null;
  size?: 'small' | 'large';
}) {
  const src = useMemo(() => symbolDataUrl(symbol), [symbol]);
  const className = `unit-symbol-icon unit-symbol-icon--${size}`;
  if (!src) return <span className={className} aria-hidden />;
  return <img className={className} src={src} alt="" />;
}
