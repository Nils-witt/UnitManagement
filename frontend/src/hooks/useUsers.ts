import { useMemo } from 'react';
import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { User } from '../api/types';

export interface UsersData {
  users: User[];
  loading: boolean;
  error: string | null;
  reloadUsers: () => Promise<void>;
}

const NO_USERS: User[] = [];

/** GET /api/users, which only administrators may call. */
export function useUsers(): UsersData {
  const api = useApi();
  const {
    data: users,
    loading,
    error,
    reload: reloadUsers,
  } = useApiQuery({ queryKey: queryKeys.users, queryFn: () => api.listUsers() }, NO_USERS);
  return useMemo(
    () => ({ users, loading, error, reloadUsers }),
    [users, loading, error, reloadUsers],
  );
}
