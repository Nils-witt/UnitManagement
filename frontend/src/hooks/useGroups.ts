import { useMemo } from 'react';
import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { Group } from '../api/types';

export interface GroupsData {
  groups: Group[];
  loading: boolean;
  error: string | null;
}

const NO_GROUPS: Group[] = [];

/** GET /api/groups, which only administrators may call. */
export function useGroups(): GroupsData {
  const api = useApi();
  const {
    data: groups,
    loading,
    error,
  } = useApiQuery({ queryKey: queryKeys.groups, queryFn: () => api.listGroups() }, NO_GROUPS);
  return useMemo(() => ({ groups, loading, error }), [groups, loading, error]);
}
