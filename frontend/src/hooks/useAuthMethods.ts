import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { AuthMethods } from '../api/types';

const PASSWORD_ONLY: AuthMethods = { oidc: false };

/** Drives the "Sign in with SSO" button on the login page. */
export function useAuthMethods(): AuthMethods {
  const api = useApi();
  const { data } = useApiQuery(
    { queryKey: queryKeys.authMethods, queryFn: () => api.getAuthMethods() },
    PASSWORD_ONLY,
  );
  return data;
}
