import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { type ApiQueryResult, useApiQuery } from './useApiQuery';
import type { SyncIdentity, SyncLogEntry, SyncRemote } from '../api/types';

const NO_IDENTITY: SyncIdentity = { serverUuid: '', publicKeyPem: '' };
const NO_REMOTES: SyncRemote[] = [];
const NO_LOGS: SyncLogEntry[] = [];

/** GET /api/sync/identity (administrators only). */
export function useSyncIdentity(): ApiQueryResult<SyncIdentity> {
  const api = useApi();
  return useApiQuery(
    { queryKey: queryKeys.syncIdentity, queryFn: () => api.getSyncIdentity() },
    NO_IDENTITY,
  );
}

/** GET /api/sync/remotes (administrators only). */
export function useSyncRemotes(): ApiQueryResult<SyncRemote[]> {
  const api = useApi();
  return useApiQuery(
    { queryKey: queryKeys.syncRemotes, queryFn: () => api.listSyncRemotes() },
    NO_REMOTES,
  );
}

/** GET /api/sync/remotes/{id}/logs while `id` is set (administrators only). */
export function useSyncRemoteLogs(id: string | null): ApiQueryResult<SyncLogEntry[]> {
  const api = useApi();
  return useApiQuery(
    {
      queryKey: queryKeys.syncRemoteLogs(id ?? ''),
      queryFn: () => api.listSyncRemoteLogs(id ?? ''),
      enabled: id !== null,
      fresh: true,
    },
    NO_LOGS,
  );
}
