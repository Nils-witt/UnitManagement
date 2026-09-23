import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '../api/queryKeys';
import type { Unit, UnitEvent } from '../api/types';
import { useApi } from './useApi';

const MIN_RETRY_MS = 1_000;
const MAX_RETRY_MS = 30_000;

/** Keeps the cached unit list live by applying changes pushed over
 * GET /api/units/events. Reconnects with backoff; after a reconnect it
 * refetches the list, since changes may have been missed while offline. */
export function useUnitEvents(): void {
  const api = useApi();
  const queryClient = useQueryClient();

  useEffect(() => {
    let socket: WebSocket | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let retryMs = MIN_RETRY_MS;
    let connectedBefore = false;
    let stopped = false;

    const apply = (event: UnitEvent) => {
      queryClient.setQueryData<Unit[]>(queryKeys.units, (units) => {
        // Nothing cached yet: the pending list fetch will include the change.
        if (!units) return units;
        const rest = units.filter((u) => u.id !== event.id);
        return event.type === 'deleted' ? rest : [...rest, event.unit];
      });
      // An open history refetches, in case the position changed.
      void queryClient.invalidateQueries({ queryKey: queryKeys.unitPositions(event.id) });
    };

    const connect = () => {
      socket = api.openUnitEvents();
      socket.onopen = () => {
        retryMs = MIN_RETRY_MS;
        // Prefix of the list and every position history.
        if (connectedBefore) void queryClient.invalidateQueries({ queryKey: ['units'] });
        connectedBefore = true;
      };
      socket.onmessage = (msg) => {
        try {
          apply(JSON.parse(msg.data as string) as UnitEvent);
        } catch {
          // A malformed message is skipped; the next resync corrects the list.
        }
      };
      socket.onclose = () => {
        socket = null;
        if (stopped) return;
        // Changes made while disconnected are fetched on the next open.
        connectedBefore = true;
        retryTimer = setTimeout(connect, retryMs);
        retryMs = Math.min(retryMs * 2, MAX_RETRY_MS);
      };
    };

    connect();
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      socket?.close();
    };
  }, [api, queryClient]);
}
