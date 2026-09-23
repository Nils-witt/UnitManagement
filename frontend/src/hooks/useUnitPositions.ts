import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { type ApiQueryResult, useApiQuery } from './useApiQuery';
import type { PositionHistoryEntry } from '../api/types';

const NO_POSITIONS: PositionHistoryEntry[] = [];

/** GET /api/units/{id}/positions; refetched when the unit event stream
 * reports a change to the unit. Disabled while `id` is null. */
export function useUnitPositions(id: string | null): ApiQueryResult<PositionHistoryEntry[]> {
  const api = useApi();
  return useApiQuery(
    {
      queryKey: queryKeys.unitPositions(id ?? ''),
      queryFn: () => api.listUnitPositions(id!),
      enabled: id != null,
      fresh: true,
    },
    NO_POSITIONS,
  );
}
