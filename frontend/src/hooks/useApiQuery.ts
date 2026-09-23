import { useCallback } from 'react';
import { type QueryKey, useQuery } from '@tanstack/react-query';
import { errorMessage } from '../lib/errors';

export interface ApiQueryOptions<T> {
  queryKey: QueryKey;
  queryFn: () => Promise<T>;
  /** False until the inputs (a username, a remote...) exist. */
  enabled?: boolean;
  /** For data behind a dialog that is edited from what it shows: always
   * refetched when opened and dropped when closed, never served from the cache. */
  fresh?: boolean;
}

export interface ApiQueryResult<T> {
  /** `empty` until the first response, so callers never handle undefined. */
  data: T;
  /** Message of the last failure, if the latest attempt failed. */
  error: string | null;
  /** True until the first response (or failure). */
  loading: boolean;
  /** Refetches and resolves when done; a failure lands in `error` instead of rejecting. */
  reload: () => Promise<void>;
}

/** The app's one way to read a resource: cached and shared by key, with the
 * `{ data, error, loading, reload }` shape the components already use. Pass a
 * module-level constant as `empty` so its identity stays stable. */
export function useApiQuery<T>(
  { queryKey, queryFn, enabled = true, fresh = false }: ApiQueryOptions<T>,
  empty: T,
): ApiQueryResult<T> {
  const { data, error, isPending, refetch } = useQuery({
    queryKey,
    queryFn,
    enabled,
    ...(fresh ? { staleTime: 0, gcTime: 0 } : {}),
  });
  const reload = useCallback(async () => {
    if (enabled) await refetch();
  }, [enabled, refetch]);
  return {
    data: data ?? empty,
    error: error ? errorMessage(error) : null,
    loading: enabled && isPending,
    reload,
  };
}
