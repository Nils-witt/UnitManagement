import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { type ApiQueryResult, useApiQuery } from './useApiQuery';
import type { ApiKey } from '../api/types';

const NO_KEYS: ApiKey[] = [];

/** GET /api/users/{id}/api-keys, which only administrators may call. */
export function useUserApiKeys(id: number): ApiQueryResult<ApiKey[]> {
  const api = useApi();
  return useApiQuery(
    { queryKey: queryKeys.userApiKeys(id), queryFn: () => api.listUserApiKeys(id), fresh: true },
    NO_KEYS,
  );
}
