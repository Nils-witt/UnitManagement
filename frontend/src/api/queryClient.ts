import { QueryClient } from '@tanstack/react-query';

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // A failed request is shown to the user as it happened; the API
        // client already handles the one retry that matters (an expired
        // session), so don't repeat 403s and 404s with backoff.
        retry: false,
        // Data changes through this UI's own actions, which reload what they
        // touched; refetching on every tab focus would only surprise.
        refetchOnWindowFocus: false,
        // Lists such as maps/users/groups are reused across pages for a while.
        staleTime: 30_000,
      },
    },
  });
}
