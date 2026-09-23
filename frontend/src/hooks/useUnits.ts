import { useMemo } from 'react';
import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { Unit } from '../api/types';

export interface UnitsData {
  units: Unit[];
  loading: boolean;
  error: string | null;
  reloadUnits: () => Promise<void>;
}

const NO_UNITS: Unit[] = [];

/** GET /api/units. */
export function useUnits(): UnitsData {
  const api = useApi();
  const {
    data: units,
    loading,
    error,
    reload: reloadUnits,
  } = useApiQuery({ queryKey: queryKeys.units, queryFn: () => api.listUnits() }, NO_UNITS);
  return useMemo(
    () => ({ units, loading, error, reloadUnits }),
    [units, loading, error, reloadUnits],
  );
}
