import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { type ApiQueryResult, useApiQuery } from './useApiQuery';
import type { PositionHistoryEntry } from '../api/types';

const NO_POSITIONS: PositionHistoryEntry[] = [];

/** GET /api/units/{id}/positions, limited to measurements from `since` and
 * up to `to` (RFC 3339) where given; refetched when the unit event stream
 * reports a change to the unit. Disabled while `id` is null. */
export function useUnitPositions(
  id: string | null,
  since: string | null = null,
  to: string | null = null,
): ApiQueryResult<PositionHistoryEntry[]> {
  const api = useApi();
  return useApiQuery(
    {
      queryKey: queryKeys.unitPositionsRange(id ?? '', since, to),
      queryFn: () => api.listUnitPositions(id!, { since: since ?? undefined, to: to ?? undefined }),
      enabled: id != null,
      fresh: true,
    },
    NO_POSITIONS,
  );
}
