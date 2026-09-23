import { type ReactNode, useMemo } from 'react';
import { ApiClient } from '../api/ApiClient';
import { useAuth } from '../hooks/useAuth.ts';
import { ApiContext } from './ApiContext.ts';

export function ApiProvider({ children }: { children: ReactNode }) {
  const { expireSession } = useAuth();

  // expireSession is stable, so this is one client for the app's lifetime.
  const instance = useMemo(
    () => new ApiClient({ onSessionExpired: expireSession }),
    [expireSession],
  );

  return <ApiContext.Provider value={instance}>{children}</ApiContext.Provider>;
}
