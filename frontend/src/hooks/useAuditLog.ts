import { useMemo } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { errorMessage } from '../lib/errors';
import type { AuditAction, AuditLogEntry } from '../api/types';

export const AUDIT_LOG_PAGE_SIZE = 100;

export interface AuditLogData {
  entries: AuditLogEntry[];
  loading: boolean;
  error: string | null;
  /** True while older entries may exist. */
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => void;
  reload: () => void;
}

/** GET /api/audit-log, newest first, a page at a time. Only administrators
 * may call it. */
export function useAuditLog(action: AuditAction | null): AuditLogData {
  const api = useApi();
  const { data, error, isPending, hasNextPage, isFetchingNextPage, fetchNextPage, refetch } =
    useInfiniteQuery({
      queryKey: queryKeys.auditLog(action),
      queryFn: ({ pageParam }) =>
        api.listAuditLog({
          action: action ?? undefined,
          before: pageParam,
          limit: AUDIT_LOG_PAGE_SIZE,
        }),
      initialPageParam: undefined as number | undefined,
      // A short page is the last one; otherwise continue after its oldest entry.
      getNextPageParam: (last) =>
        last.length < AUDIT_LOG_PAGE_SIZE ? undefined : last[last.length - 1].id,
      // New entries appear with every change, so always show the latest.
      staleTime: 0,
    });

  return useMemo(
    () => ({
      entries: data?.pages.flat() ?? [],
      loading: isPending,
      error: error ? errorMessage(error) : null,
      hasMore: hasNextPage,
      loadingMore: isFetchingNextPage,
      loadMore: () => void fetchNextPage(),
      reload: () => void refetch(),
    }),
    [data, isPending, error, hasNextPage, isFetchingNextPage, fetchNextPage, refetch],
  );
}
