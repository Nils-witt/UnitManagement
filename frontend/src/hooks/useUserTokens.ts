import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { type ApiQueryResult, useApiQuery } from './useApiQuery';
import type { ApiToken } from '../api/types';

const NO_TOKENS: ApiToken[] = [];

/** GET /api/users/{id}/tokens, which only administrators may call. */
export function useUserTokens(id: number): ApiQueryResult<ApiToken[]> {
  const api = useApi();
  return useApiQuery(
    { queryKey: queryKeys.userTokens(id), queryFn: () => api.listUserTokens(id), fresh: true },
    NO_TOKENS,
  );
}
