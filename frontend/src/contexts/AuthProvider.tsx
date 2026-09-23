import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { ApiClient } from '../api/ApiClient.ts';
import type { User } from '../api/types.ts';
import RouteFallback from '../components/RouteFallback.tsx';
import { AuthContext, type AuthState } from './AuthContext.ts';

/** A client for the auth endpoints; it must not report its own 401s (wrong
 * password, no session yet) as an expired session. */
const anonymousApi = new ApiClient();

export function AuthProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [user, setUserState] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [sessionMessage, setSessionMessage] = useState<string | null>(null);
  // Mirrors `user` for expireSession, which must stay stable across renders.
  const userRef = useRef<User | null>(null);
  const setUser = useCallback((next: User | null) => {
    userRef.current = next;
    setUserState(next);
  }, []);

  // The session cookie is HttpOnly, so the only way to know whether it is
  // still valid is to ask the server once on load.
  useEffect(() => {
    let cancelled = false;
    anonymousApi
      .me()
      .then((me) => {
        if (!cancelled) setUser(me);
      })
      .catch((err: unknown) => console.error(err))
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [setUser]);

  const login = useCallback(
    async (username: string, password: string) => {
      const loggedIn = await anonymousApi.login(username, password);
      setSessionMessage(null);
      setUser(loggedIn);
      return loggedIn;
    },
    [setUser],
  );

  const endSession = useCallback(() => {
    setUser(null);
    // Cached API data belongs to the user who fetched it: without this the
    // next login would briefly see the previous user's data.
    queryClient.clear();
  }, [queryClient, setUser]);

  const logout = useCallback(async () => {
    try {
      await anonymousApi.logout();
    } catch (err) {
      // The cookie is cleared client-side regardless; the server session
      // expires on its own.
      console.error(err);
    }
    endSession();
    setSessionMessage(null);
  }, [endSession]);

  const expireSession = useCallback(() => {
    // Late failures of requests from a session that already ended must not
    // announce an expiry that didn't happen.
    if (!userRef.current) return;
    endSession();
    setSessionMessage(t('auth.sessionExpired'));
  }, [endSession, t]);

  const clearSessionMessage = useCallback(() => setSessionMessage(null), []);

  const value = useMemo<AuthState>(
    () => ({
      user,
      isAuthenticated: !!user,
      sessionMessage,
      login,
      logout,
      clearSessionMessage,
      expireSession,
    }),
    [user, sessionMessage, login, logout, clearSessionMessage, expireSession],
  );

  if (loading) return <RouteFallback />;

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
